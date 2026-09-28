package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnauthorized = errors.New("unauthorized: missing or invalid credentials")
	ErrForbidden    = errors.New("forbidden: insufficient permissions")
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Session stores an active authenticated session.
type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

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

// Manager handles API authentication, RBAC, session lifecycle, enrollment tokens, and audit logging.
type Manager struct {
	mu           sync.RWMutex
	adminTokens  map[string]Role   // static token -> role
	sessions     map[string]*Session // sessionToken -> Session
	enrollTokens map[string]string // token -> namespace
	auditLogs    []AuditEvent
	maxAuditLogs int
}

func NewManager(defaultAdminToken, defaultEnrollToken string) *Manager {
	m := &Manager{
		adminTokens:  make(map[string]Role),
		sessions:     make(map[string]*Session),
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

// Login verifies credentials and creates a new session.
func (m *Manager) Login(username, secret string, clientIP string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check against static admin tokens
	role, tokenMatches := m.adminTokens[secret]
	validUser := (username == "admin" || username == "operator" || username == "") && tokenMatches

	// Also allow username "admin" with secret equal to any registered admin token
	if !validUser {
		for tok, r := range m.adminTokens {
			if subtle.ConstantTimeCompare([]byte(tok), []byte(secret)) == 1 {
				validUser = true
				role = r
				break
			}
		}
	}

	if !validUser {
		m.recordAuditLocked("anonymous", "none", "LOGIN", "auth/login", "BLOCKED", "Invalid credentials attempted", clientIP)
		return nil, ErrUnauthorized
	}

	if username == "" {
		username = "admin"
	}

	// Generate secure session token
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	sessionToken := "sess_" + hex.EncodeToString(buf)

	sess := &Session{
		Token:     sessionToken,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	m.sessions[sessionToken] = sess
	m.recordAuditLocked(username, string(role), "LOGIN", "auth/login", "SUCCESS", "User logged in successfully", clientIP)
	return sess, nil
}

// Logout invalidates a session token.
func (m *Manager) Logout(sessionToken string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionToken)
}

// ValidateEnrollToken verifies if the enrollment token is valid for a given namespace.
func (m *Manager) ValidateEnrollToken(token, namespace string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.enrollTokens) == 0 {
		return true
	}

	expectedNs, exists := m.enrollTokens[token]
	if !exists {
		for tok := range m.enrollTokens {
			if subtle.ConstantTimeCompare([]byte(tok), []byte(token)) == 1 {
				return true
			}
		}
		return false
	}

	return expectedNs == "" || expectedNs == namespace || namespace == "default"
}

// AuthenticateRequest extracts and verifies session or bearer token from HTTP request.
func (m *Manager) AuthenticateRequest(r *http.Request) (Role, string, error) {
	var token string

	// 1. Check Authorization Bearer header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = parts[1]
		}
	}

	// 2. Check X-API-Key header
	if token == "" {
		token = r.Header.Get("X-API-Key")
	}

	// 3. Check session cookie
	if token == "" {
		if c, err := r.Cookie("torana_session"); err == nil {
			token = c.Value
		}
	}

	// 4. Check query parameter
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	if token == "" {
		return "", "", ErrUnauthorized
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check active dynamic sessions
	if sess, ok := m.sessions[token]; ok {
		if time.Now().UTC().After(sess.ExpiresAt) {
			return "", "", ErrUnauthorized
		}
		return sess.Role, sess.Username, nil
	}

	// Check static configured admin tokens
	if role, ok := m.adminTokens[token]; ok {
		return role, "admin@key", nil
	}

	return "", "", ErrUnauthorized
}

// RecordAudit logs an administrative event.
func (m *Manager) RecordAudit(actor, role, action, resource, status, details, clientIP string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordAuditLocked(actor, role, action, resource, status, details, clientIP)
}

func (m *Manager) recordAuditLocked(actor, role, action, resource, status, details, clientIP string) {
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
