package fleet

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NodeHealthStatus represents the operational status of a node.
type NodeHealthStatus string

const (
	StatusHealthy      NodeHealthStatus = "HEALTHY"
	StatusLagging      NodeHealthStatus = "LAGGING"
	StatusDegraded     NodeHealthStatus = "DEGRADED"
	StatusDisconnected NodeHealthStatus = "DISCONNECTED"
)

// NodeRecord contains runtime state, telemetry, and rollout progress of a gateway instance.
type NodeRecord struct {
	NodeID               string           `json:"node_id"`
	Namespace            string           `json:"namespace"`
	Version              string           `json:"version"`
	RemoteAddr           string           `json:"remote_addr"`
	ConnectedAt          time.Time        `json:"connected_at"`
	LastHeartbeat        time.Time        `json:"last_heartbeat"`
	CurrentConfigVersion uint64           `json:"current_config_version"`
	TargetConfigVersion  uint64           `json:"target_config_version"`
	LastAckStatus        string           `json:"last_ack_status"` // "ACK", "NACK", "PENDING"
	LastAckTime          time.Time        `json:"last_ack_time"`
	LastNackReason       string           `json:"last_nack_reason,omitempty"`
	HealthStatus         NodeHealthStatus `json:"health_status"`
	ActiveConnections    int64            `json:"active_connections"`
	ActiveStreams        int64            `json:"active_streams"`
	MemoryAllocatedBytes int64            `json:"memory_allocated_bytes"`
	CPUUsagePermille     int64            `json:"cpu_usage_permille"`
	SupportedProtocols   []string         `json:"supported_protocols,omitempty"`
}

// TenantUsageRecord stores aggregated usage for analytics.
type TenantUsageRecord struct {
	TenantID         string    `json:"tenant_id"`
	RouteID          string    `json:"route_id"`
	Model            string    `json:"model"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalRequests    int64     `json:"total_requests"`
	DurationMsSum    int64     `json:"duration_ms_sum"`
	LastReported     time.Time `json:"last_reported"`
}

type activeStream struct {
	nodeID  string
	msgChan chan *controlplanev1.ControlMessage
}

// Manager coordinates gateway node registrations, active streaming channels, and fleet telemetry.
type Manager struct {
	mu           sync.RWMutex
	nodes        map[string]*NodeRecord
	streams      map[string]*activeStream
	usage        map[string]*TenantUsageRecord // tenant:model:route -> aggregate
	logger       *slog.Logger
	onNodeEvent  func(event string, node *NodeRecord)
}

func NewManager(logger *slog.Logger) *Manager {
	m := &Manager{
		nodes:   make(map[string]*NodeRecord),
		streams: make(map[string]*activeStream),
		usage:   make(map[string]*TenantUsageRecord),
		logger:  logger,
	}

	// Background ticker to flag disconnected or lagging nodes
	go m.healthCheckLoop()

	return m
}

func (m *Manager) SetNodeEventHandler(handler func(event string, node *NodeRecord)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onNodeEvent = handler
}

// RegisterEnrollment creates or updates a node enrollment record.
func (m *Manager) RegisterEnrollment(nodeID, namespace, version, remoteAddr string, targetVersion uint64) *NodeRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		node = &NodeRecord{
			NodeID:               nodeID,
			Namespace:            namespace,
			Version:              version,
			RemoteAddr:           remoteAddr,
			ConnectedAt:          time.Now().UTC(),
			LastHeartbeat:        time.Now().UTC(),
			TargetConfigVersion:  targetVersion,
			CurrentConfigVersion: 0,
			LastAckStatus:        "PENDING",
			HealthStatus:         StatusHealthy,
		}
		m.nodes[nodeID] = node
	} else {
		node.Namespace = namespace
		node.Version = version
		node.RemoteAddr = remoteAddr
		node.LastHeartbeat = time.Now().UTC()
		node.TargetConfigVersion = targetVersion
		if node.HealthStatus == StatusDisconnected {
			node.HealthStatus = StatusHealthy
		}
	}

	m.emitEventLocked("node_enrolled", node)
	return node
}

// RegisterStream attaches an active gRPC stream channel to a node.
func (m *Manager) RegisterStream(nodeID string, bufSize int) chan *controlplanev1.ControlMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bufSize <= 0 {
		bufSize = 32
	}

	// Close old stream channel if exists
	if old, ok := m.streams[nodeID]; ok {
		close(old.msgChan)
	}

	msgChan := make(chan *controlplanev1.ControlMessage, bufSize)
	m.streams[nodeID] = &activeStream{
		nodeID:  nodeID,
		msgChan: msgChan,
	}

	if node, ok := m.nodes[nodeID]; ok {
		node.HealthStatus = StatusHealthy
		node.LastHeartbeat = time.Now().UTC()
		m.emitEventLocked("node_stream_opened", node)
	}

	return msgChan
}

// UnregisterStream removes an active stream when gRPC stream disconnects.
func (m *Manager) UnregisterStream(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.streams, nodeID)
	if node, ok := m.nodes[nodeID]; ok {
		node.HealthStatus = StatusDisconnected
		m.emitEventLocked("node_stream_closed", node)
	}
}

// RecordHello updates node capabilities reported on stream open.
func (m *Manager) RecordHello(nodeID, version string, protocols []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node, ok := m.nodes[nodeID]; ok {
		if version != "" {
			node.Version = version
		}
		node.SupportedProtocols = protocols
		node.LastHeartbeat = time.Now().UTC()
		m.emitEventLocked("node_hello", node)
	}
}

// RecordAck updates config version acknowledged by node.
func (m *Manager) RecordAck(nodeID string, version uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node, ok := m.nodes[nodeID]; ok {
		node.CurrentConfigVersion = version
		node.LastAckStatus = "ACK"
		node.LastAckTime = time.Now().UTC()
		node.LastNackReason = ""

		if node.TargetConfigVersion == 0 || node.CurrentConfigVersion >= node.TargetConfigVersion {
			node.HealthStatus = StatusHealthy
		}

		m.emitEventLocked("node_ack", node)
	}
}

// RecordNack updates config version rejected by node.
func (m *Manager) RecordNack(nodeID string, version uint64, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node, ok := m.nodes[nodeID]; ok {
		node.LastAckStatus = "NACK"
		node.LastAckTime = time.Now().UTC()
		node.LastNackReason = reason
		node.HealthStatus = StatusDegraded
		m.emitEventLocked("node_nack", node)
	}
}

// RecordHeartbeat updates runtime metrics.
func (m *Manager) RecordHeartbeat(nodeID string, activeConns, activeStreams, memBytes, cpuPermille int64, currentVer uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node, ok := m.nodes[nodeID]; ok {
		node.LastHeartbeat = time.Now().UTC()
		node.ActiveConnections = activeConns
		node.ActiveStreams = activeStreams
		node.MemoryAllocatedBytes = memBytes
		node.CPUUsagePermille = cpuPermille
		if currentVer > 0 {
			node.CurrentConfigVersion = currentVer
		}

		// Re-evaluate lagging status
		if node.TargetConfigVersion > 0 && node.CurrentConfigVersion < node.TargetConfigVersion {
			if node.HealthStatus != StatusDegraded {
				node.HealthStatus = StatusLagging
			}
		} else if node.HealthStatus == StatusLagging || node.HealthStatus == StatusDisconnected {
			node.HealthStatus = StatusHealthy
		}
	}
}

// RecordUsage aggregates tenant usage reports.
func (m *Manager) RecordUsage(reports []*controlplanev1.TenantUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	for _, r := range reports {
		key := fmt.Sprintf("%s:%s:%s", r.TenantId, r.Model, r.RouteId)
		agg, exists := m.usage[key]
		if !exists {
			agg = &TenantUsageRecord{
				TenantID: r.TenantId,
				RouteID:  r.RouteId,
				Model:    r.Model,
			}
			m.usage[key] = agg
		}
		agg.PromptTokens += r.PromptTokens
		agg.CompletionTokens += r.CompletionTokens
		agg.TotalRequests += r.TotalRequests
		agg.DurationMsSum += r.DurationMsSum
		agg.LastReported = now
	}
}

// BroadcastSnapshot sends a snapshot message to all active stream connections.
func (m *Manager) BroadcastSnapshot(snap *controlplanev1.Snapshot) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := &controlplanev1.ControlMessage{
		MessageId: fmt.Sprintf("msg-snap-%d-%d", snap.ConfigVersion, time.Now().UnixNano()),
		Timestamp: timestamppb.Now(),
		Payload: &controlplanev1.ControlMessage_Snapshot{
			Snapshot: snap,
		},
	}

	pushed := 0
	for _, stream := range m.streams {
		select {
		case stream.msgChan <- msg:
			pushed++
		default:
			m.logger.Warn("dropping snapshot message, node stream channel full", "node_id", stream.nodeID)
		}
	}

	// Update target config version for all known nodes
	for _, node := range m.nodes {
		node.TargetConfigVersion = snap.ConfigVersion
		node.LastAckStatus = "PENDING"
		if node.HealthStatus == StatusHealthy {
			node.HealthStatus = StatusLagging
		}
	}

	return pushed
}

// BroadcastRevocation sends a revocation message to all connected data planes immediately.
func (m *Manager) BroadcastRevocation(rev *controlplanev1.Revocation) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msg := &controlplanev1.ControlMessage{
		MessageId: fmt.Sprintf("msg-rev-%d", time.Now().UnixNano()),
		Timestamp: timestamppb.Now(),
		Payload: &controlplanev1.ControlMessage_Revocation{
			Revocation: rev,
		},
	}

	pushed := 0
	for _, stream := range m.streams {
		select {
		case stream.msgChan <- msg:
			pushed++
		default:
			m.logger.Warn("dropping revocation message, channel full", "node_id", stream.nodeID)
		}
	}

	return pushed
}

// GetNodes returns snapshot of all registered nodes.
func (m *Manager) GetNodes() []*NodeRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*NodeRecord, 0, len(m.nodes))
	for _, n := range m.nodes {
		copied := *n
		res = append(res, &copied)
	}
	return res
}

// GetUsageSummary returns aggregated usage statistics.
func (m *Manager) GetUsageSummary() []*TenantUsageRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*TenantUsageRecord, 0, len(m.usage))
	for _, u := range m.usage {
		copied := *u
		res = append(res, &copied)
	}
	return res
}

// FleetStats calculates summary metrics across all nodes.
type FleetStats struct {
	TotalNodes          int   `json:"total_nodes"`
	ConnectedNodes      int   `json:"connected_nodes"`
	HealthyNodes        int   `json:"healthy_nodes"`
	LaggingNodes        int   `json:"lagging_nodes"`
	DegradedNodes       int   `json:"degraded_nodes"`
	TotalActiveConns    int64 `json:"total_active_conns"`
	TotalMemoryBytes    int64 `json:"total_memory_bytes"`
	TotalTokensRecorded int64 `json:"total_tokens_recorded"`
}

func (m *Manager) GetFleetStats() FleetStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats FleetStats
	stats.TotalNodes = len(m.nodes)
	stats.ConnectedNodes = len(m.streams)

	for _, n := range m.nodes {
		switch n.HealthStatus {
		case StatusHealthy:
			stats.HealthyNodes++
		case StatusLagging:
			stats.LaggingNodes++
		case StatusDegraded:
			stats.DegradedNodes++
		}
		stats.TotalActiveConns += n.ActiveConnections
		stats.TotalMemoryBytes += n.MemoryAllocatedBytes
	}

	for _, u := range m.usage {
		stats.TotalTokensRecorded += u.PromptTokens + u.CompletionTokens
	}

	return stats
}

func (m *Manager) emitEventLocked(event string, node *NodeRecord) {
	if m.onNodeEvent != nil && node != nil {
		copied := *node
		go m.onNodeEvent(event, &copied)
	}
}

func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now().UTC()
		for id, node := range m.nodes {
			// If not in active streams or no heartbeat for 15s, mark disconnected
			_, isConnected := m.streams[id]
			if !isConnected || now.Sub(node.LastHeartbeat) > 15*time.Second {
				if node.HealthStatus != StatusDisconnected {
					node.HealthStatus = StatusDisconnected
					m.emitEventLocked("node_disconnected", node)
				}
			}
		}
		m.mu.Unlock()
	}
}
