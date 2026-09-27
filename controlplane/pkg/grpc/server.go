package grpc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"github.com/phaselume/torana/controlplane/pkg/auth"
	"github.com/phaselume/torana/controlplane/pkg/fleet"
	"github.com/phaselume/torana/controlplane/pkg/state"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements controlplanev1.ControlPlaneServiceServer for the gateway fleet.
type Server struct {
	controlplanev1.UnimplementedControlPlaneServiceServer
	clusterID string
	state     *state.Store
	fleet     *fleet.Manager
	auth      *auth.Manager
	logger    *slog.Logger
}

// NewServer creates a new gRPC control plane service server.
func NewServer(
	clusterID string,
	state *state.Store,
	fleet *fleet.Manager,
	auth *auth.Manager,
	logger *slog.Logger,
) *Server {
	if clusterID == "" {
		clusterID = "torana-cluster-main"
	}
	return &Server{
		clusterID: clusterID,
		state:     state,
		fleet:     fleet,
		auth:      auth,
		logger:    logger,
	}
}

// Enroll handles gateway node registration and identity assignment.
func (s *Server) Enroll(ctx context.Context, req *controlplanev1.EnrollRequest) (*controlplanev1.EnrollResponse, error) {
	s.logger.Info("gateway node requesting enrollment",
		"node_id", req.NodeId,
		"namespace", req.Namespace,
		"version", req.Version,
	)

	// Validate enroll token
	if !s.auth.ValidateEnrollToken(req.EnrollToken, req.Namespace) {
		s.logger.Warn("enrollment rejected: invalid enroll token",
			"node_id", req.NodeId,
			"namespace", req.Namespace,
		)
		s.auth.RecordAudit(req.NodeId, "node", "ENROLL", req.Namespace, "BLOCKED", "Invalid enroll token", "")
		return &controlplanev1.EnrollResponse{
			Accepted:     false,
			ErrorMessage: "invalid or expired enrollment token",
		}, nil
	}

	targetVer := s.state.ConfigVersion()
	s.fleet.RegisterEnrollment(req.NodeId, req.Namespace, req.Version, "remote", targetVer)
	s.auth.RecordAudit(req.NodeId, "node", "ENROLL", req.Namespace, "SUCCESS", "Node enrolled successfully", "")

	return &controlplanev1.EnrollResponse{
		Accepted:   true,
		ClusterId:  s.clusterID,
		EnrolledAt: timestamppb.Now(),
	}, nil
}

// Stream establishes the persistent bidirectional gRPC control channel.
func (s *Server) Stream(srv controlplanev1.ControlPlaneService_StreamServer) error {
	ctx := srv.Context()

	// 1. Read first message from node
	firstMsg, err := srv.Recv()
	if err != nil {
		return fmt.Errorf("failed to read stream initial handshake: %w", err)
	}

	nodeID := firstMsg.NodeId
	if nodeID == "" {
		nodeID = fmt.Sprintf("gateway-anon-%d", time.Now().UnixNano()%100000)
	}

	s.logger.Info("data plane node opened control stream",
		"node_id", nodeID,
		"namespace", firstMsg.Namespace,
	)

	// Handle initial payload if Hello
	if hello := firstMsg.GetHello(); hello != nil {
		s.fleet.RecordHello(nodeID, hello.Version, hello.SupportedProtocols)
	}

	// Register active stream channel
	msgChan := s.fleet.RegisterStream(nodeID, 64)
	defer s.fleet.UnregisterStream(nodeID)

	// 2. Immediately push current configuration snapshot to connecting node
	snap := s.state.CurrentSnapshot()
	if snap != nil {
		initMsg := &controlplanev1.ControlMessage{
			MessageId: fmt.Sprintf("init-snap-%d-%s", snap.ConfigVersion, nodeID),
			Timestamp: timestamppb.Now(),
			Payload: &controlplanev1.ControlMessage_Snapshot{
				Snapshot: snap,
			},
		}
		if err := srv.Send(initMsg); err != nil {
			s.logger.Error("failed to send initial snapshot to node", "node_id", nodeID, "error", err)
			return err
		}
		s.logger.Info("dispatched initial snapshot to node", "node_id", nodeID, "version", snap.ConfigVersion)
	}

	errChan := make(chan error, 2)

	// 3. Sender loop: outgoing messages from control plane to data plane
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgChan:
				if !ok {
					return
				}
				if err := srv.Send(msg); err != nil {
					errChan <- err
					return
				}
			}
		}
	}()

	// 4. Receiver loop: incoming messages from data plane to control plane
	go func() {
		for {
			msg, err := srv.Recv()
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				return
			}

			s.handleNodeMessage(msg)
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("node control stream closed by context", "node_id", nodeID)
		return ctx.Err()
	case err := <-errChan:
		s.logger.Warn("node control stream terminated", "node_id", nodeID, "error", err)
		return err
	}
}

func (s *Server) handleNodeMessage(msg *controlplanev1.NodeMessage) {
	nodeID := msg.NodeId

	switch p := msg.Payload.(type) {
	case *controlplanev1.NodeMessage_Hello:
		s.logger.Info("received Hello from node", "node_id", nodeID, "version", p.Hello.Version)
		s.fleet.RecordHello(nodeID, p.Hello.Version, p.Hello.SupportedProtocols)

	case *controlplanev1.NodeMessage_Ack:
		s.logger.Info("received ACK from data plane", "node_id", nodeID, "version", p.Ack.AppliedConfigVersion)
		s.fleet.RecordAck(nodeID, p.Ack.AppliedConfigVersion)

	case *controlplanev1.NodeMessage_Nack:
		s.logger.Error("received NACK from data plane",
			"node_id", nodeID,
			"rejected_version", p.Nack.RejectedConfigVersion,
			"error_code", p.Nack.ErrorCode,
			"error_message", p.Nack.ErrorMessage,
		)
		s.fleet.RecordNack(nodeID, p.Nack.RejectedConfigVersion, p.Nack.ErrorMessage)

	case *controlplanev1.NodeMessage_Heartbeat:
		s.fleet.RecordHeartbeat(
			nodeID,
			p.Heartbeat.ActiveConnections,
			p.Heartbeat.ActiveStreams,
			p.Heartbeat.MemoryAllocatedBytes,
			p.Heartbeat.CpuUsagePermille,
			p.Heartbeat.CurrentConfigVersion,
		)

	case *controlplanev1.NodeMessage_UsageReport:
		s.logger.Info("received telemetry usage report from data plane",
			"node_id", nodeID,
			"tenant_records", len(p.UsageReport.TenantReports),
		)
		s.fleet.RecordUsage(p.UsageReport.TenantReports)

	case *controlplanev1.NodeMessage_PluginStatus:
		s.logger.Info("received plugin status from node",
			"node_id", nodeID,
			"plugin_id", p.PluginStatus.PluginId,
			"status", p.PluginStatus.Status,
		)
	}
}
