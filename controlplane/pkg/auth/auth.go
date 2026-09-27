package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden: insufficient permissions")
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// AuditEvent records a security or operational event.
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Role      string    `json:"role"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Status    string    `json:"status"` // "SUCCESS", "FAILURE", "BLOCKED"
	Details   string    `json:"details,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
}

// Manager handles API authentication, RBAC, enrollment tokens, and audit logging.
type Manager struct {
	mu           sync.RWMutex
	adminTokens  map[string]Role   // token -> role
	enrollTokens map[string]string // token -> namespace
	auditLogs    []AuditEvent
	maxAuditLogs int
}

func NewManager(defaultAdminToken, defaultEnrollToken string) *Manager {
	m := &Manager{
		adminTokens:  make(map[string]Role),
		enrollTokens: make(map[string]string),
		auditLogs:    make([]AuditEvent, 0, 500),
		maxAuditLogs: 500,
	}

	if defaultAdminToken != "" {
		m.adminTokens[defaultAdminToken] = RoleAdmin
	} else {
		m.adminTokens["torana-admin-secret-key"] = RoleAdmin
	}

	if defaultEnrollToken != "" {
		m.enrollTokens[defaultEnrollToken] = "default"
	} else {
		m.enrollTokens["torana-enroll-dev-secret"] = "default"
	}

	return m
}

// ValidateEnrollToken verifies if the enrollment token is valid for a given namespace.
func (m *Manager) ValidateEnrollToken(token, namespace string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If empty token registry, allow in dev mode
	if len(m.enrollTokens) == 0 {
		return true
	}

	expectedNs, exists := m.enrollTokens[token]
	if !exists {
		// Also allow token if matched against any registered enroll token in dev
		for tok := range m.enrollTokens {
			if subtle.ConstantTimeCompare([]byte(tok), []byte(token)) == 1 {
				return true
			}
		}
		return false
	}

	return expectedNs == "" || expectedNs == namespace || namespace == "default"
}

// AuthenticateRequest extracts and verifies bearer token from HTTP request.
func (m *Manager) AuthenticateRequest(r *http.Request) (Role, string, error) {
	authHeader := r.Header.Get("Authorization")
	apiKeyHeader := r.Header.Get("X-API-Key")

	var token string
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = parts[1]
		}
	}
	if token == "" && apiKeyHeader != "" {
		token = apiKeyHeader
	}

	// For local dev convenience, allow cookie or query param if provided
	if token == "" {
		if c, err := r.Cookie("torana_token"); err == nil {
			token = c.Value
		}
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	// If no token provided, allow dev default with RoleAdmin for localhost UI
	if token == "" {
		return RoleAdmin, "admin@local", nil
	}

	m.mu.RLock()
	role, exists := m.adminTokens[token]
	m.mu.RUnlock()

	if !exists {
		return "", "", ErrUnauthorized
	}

	return role, "operator", nil
}

// RecordAudit logs an administrative event.
func (m *Manager) RecordAudit(actor, role, action, resource, status, details, clientIP string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	event := AuditEvent{
		ID:        time.Now().Format("20060102150405.000000"),
		Timestamp: time.Now().UTC(),
		Actor:     actor,
		Role:      role,
		Action:    action,
		Resource:  resource,
		Status:    status,
		Details:   details,
		ClientIP:  clientIP,
	}

	m.auditLogs = append(m.auditLogs, event)
	if len(m.auditLogs) > m.maxAuditLogs {
		m.auditLogs = m.auditLogs[len(m.auditLogs)-m.maxAuditLogs:]
	}
}

// GetAuditLogs returns the recent audit logs (newest first).
func (m *Manager) GetAuditLogs(limit int) []AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.auditLogs)
	if total == 0 {
		return []AuditEvent{}
	}

	if limit <= 0 || limit > total {
		limit = total
	}

	result := make([]AuditEvent, 0, limit)
	for i := total - 1; i >= total-limit; i-- {
		result = append(result, m.auditLogs[i])
	}
	return result
}
