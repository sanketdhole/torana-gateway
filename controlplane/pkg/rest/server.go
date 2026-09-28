package rest

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"github.com/phaselume/torana/controlplane/pkg/auth"
	"github.com/phaselume/torana/controlplane/pkg/crypto"
	"github.com/phaselume/torana/controlplane/pkg/fleet"
	"github.com/phaselume/torana/controlplane/pkg/state"
)

// Server provides the REST API and serves the React dashboard.
type Server struct {
	state      *state.Store
	fleet      *fleet.Manager
	auth       *auth.Manager
	signer     *crypto.Signer
	logger     *slog.Logger
	uiDir      string
	embeddedFS fs.FS

	sseMu      sync.Mutex
	sseClients map[chan string]struct{}
}

// NewServer creates a new REST API server.
func NewServer(
	state *state.Store,
	fleetMgr *fleet.Manager,
	auth *auth.Manager,
	signer *crypto.Signer,
	logger *slog.Logger,
	uiDir string,
	embeddedFS fs.FS,
) *Server {
	s := &Server{
		state:      state,
		fleet:      fleetMgr,
		auth:       auth,
		signer:     signer,
		logger:     logger,
		uiDir:      uiDir,
		embeddedFS: embeddedFS,
		sseClients: make(map[chan string]struct{}),
	}

	// Forward fleet events to connected SSE clients
	fleetMgr.SetNodeEventHandler(func(event string, node *fleet.NodeRecord) {
		payload, err := json.Marshal(map[string]any{
			"type":      event,
			"node":      node,
			"timestamp": time.Now().UTC(),
		})
		if err == nil {
			s.broadcastSSE(string(payload))
		}
	})

	return s
}

// Handler returns the HTTP handler with all registered endpoints and middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Authentication Endpoints
	mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleAuthMe)

	// REST API Endpoints
	mux.HandleFunc("GET /api/status", s.handleGetStatus)
	mux.HandleFunc("GET /api/nodes", s.handleGetNodes)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("GET /api/config/history", s.handleGetConfigHistory)

	// Direct CRUD Endpoints
	mux.HandleFunc("POST /api/config/routes", s.handleUpsertRoute)
	mux.HandleFunc("DELETE /api/config/routes", s.handleDeleteRoute)
	mux.HandleFunc("POST /api/config/policies", s.handleUpsertPolicy)
	mux.HandleFunc("DELETE /api/config/policies", s.handleDeletePolicy)
	mux.HandleFunc("POST /api/config/upstreams", s.handleUpsertUpstream)
	mux.HandleFunc("DELETE /api/config/upstreams", s.handleDeleteUpstream)

	// Snapshot Publisher & Rollback
	mux.HandleFunc("POST /api/config/publish", s.handlePublishConfig)
	mux.HandleFunc("POST /api/config/rollback", s.handleRollbackConfig)

	// Revocation & Telemetry
	mux.HandleFunc("GET /api/revocations", s.handleGetRevocations)
	mux.HandleFunc("POST /api/revocations", s.handleAddRevocation)
	mux.HandleFunc("GET /api/usage", s.handleGetUsage)
	mux.HandleFunc("GET /api/audit", s.handleGetAudit)
	mux.HandleFunc("GET /api/events", s.handleSSE)

	// React Static / SPA Handler
	mux.HandleFunc("/", s.handleStaticOrSPA)

	return s.withCorsAndSecurityHeaders(mux)
}

func (s *Server) withCorsAndSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, map[string]string{"error": msg})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid login json payload")
		return
	}

	secret := req.Password
	if secret == "" {
		secret = req.Token
	}

	sess, err := s.auth.Login(req.Username, secret, r.RemoteAddr)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "invalid username, password, or admin token")
		return
	}

	// Set secure HTTP session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "torana_session",
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	s.writeJSON(w, http.StatusOK, map[string]any{
		"token":      sess.Token,
		"username":   sess.Username,
		"role":       sess.Role,
		"expires_at": sess.ExpiresAt,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	var token string
	if authHdr := r.Header.Get("Authorization"); strings.HasPrefix(authHdr, "Bearer ") {
		token = strings.TrimPrefix(authHdr, "Bearer ")
	} else if c, err := r.Cookie("torana_session"); err == nil {
		token = c.Value
	}

	if token != "" {
		s.auth.Logout(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "torana_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      actor,
		"role":          role,
	})
}

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.fleet.GetFleetStats()
	snap := s.state.CurrentSnapshot()

	var routeCount, upstreamCount, policyCount int
	if snap != nil {
		routeCount = len(snap.Routes)
		upstreamCount = len(snap.Upstreams)
		policyCount = len(snap.Policies)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"service":         "torana-controlplane",
		"status":          "OPERATIONAL",
		"public_key_hex":  s.signer.PublicKeyHex(),
		"config_version":  s.state.ConfigVersion(),
		"routes_count":    routeCount,
		"upstreams_count": upstreamCount,
		"policies_count":  policyCount,
		"fleet_stats":     stats,
		"timestamp":       time.Now().UTC(),
	})
}

func (s *Server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.fleet.GetNodes()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	schema := s.state.GetSchema()
	s.writeJSON(w, http.StatusOK, schema)
}

func (s *Server) handleGetConfigHistory(w http.ResponseWriter, r *http.Request) {
	history := s.state.GetHistory()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"history": history,
	})
}

// handleUpsertRoute creates or updates a route and publishes snapshot
func (s *Server) handleUpsertRoute(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	var route controlplanev1.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid route payload: "+err.Error())
		return
	}
	if route.Id == "" || route.Path == "" {
		s.writeError(w, http.StatusBadRequest, "route id and path are required")
		return
	}

	snap, err := s.state.UpsertRoute(&route, actor)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "UPSERT_ROUTE", route.Id, "SUCCESS", fmt.Sprintf("Path: %s, Upstream: %s", route.Path, route.UpstreamId), r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "route updated and broadcast to fleet",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
		"route":          route,
	})
}

// handleDeleteRoute deletes a route by id
func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	routeID := r.URL.Query().Get("id")
	if routeID == "" {
		var req struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		routeID = req.ID
	}
	if routeID == "" {
		s.writeError(w, http.StatusBadRequest, "id parameter required")
		return
	}

	snap, err := s.state.DeleteRoute(routeID, actor)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "DELETE_ROUTE", routeID, "SUCCESS", "Route removed", r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "route deleted and broadcast to fleet",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
	})
}

// handleUpsertPolicy creates or updates a CEL policy
func (s *Server) handleUpsertPolicy(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	var policy controlplanev1.Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid policy payload: "+err.Error())
		return
	}
	if policy.Id == "" {
		s.writeError(w, http.StatusBadRequest, "policy id is required")
		return
	}

	snap, err := s.state.UpsertPolicy(&policy, actor)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "UPSERT_POLICY", policy.Id, "SUCCESS", fmt.Sprintf("Type: %s, Action: %s", policy.Type, policy.Action), r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "policy updated and broadcast to fleet",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
		"policy":         policy,
	})
}

// handleDeletePolicy removes a policy by ID
func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	policyID := r.URL.Query().Get("id")
	if policyID == "" {
		var req struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		policyID = req.ID
	}
	if policyID == "" {
		s.writeError(w, http.StatusBadRequest, "id parameter required")
		return
	}

	snap, err := s.state.DeletePolicy(policyID, actor)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "DELETE_POLICY", policyID, "SUCCESS", "Policy removed", r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "policy deleted and broadcast to fleet",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
	})
}

// handleUpsertUpstream creates or updates an upstream cluster
func (s *Server) handleUpsertUpstream(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	var upstream controlplanev1.Upstream
	if err := json.NewDecoder(r.Body).Decode(&upstream); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid upstream payload: "+err.Error())
		return
	}
	if upstream.Id == "" {
		s.writeError(w, http.StatusBadRequest, "upstream id is required")
		return
	}

	snap, err := s.state.UpsertUpstream(&upstream, actor)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "UPSERT_UPSTREAM", upstream.Id, "SUCCESS", fmt.Sprintf("Protocol: %s, Endpoints: %d", upstream.Protocol, len(upstream.Endpoints)), r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "upstream updated and broadcast to fleet",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
		"upstream":       upstream,
	})
}

// handleDeleteUpstream removes an upstream by ID
func (s *Server) handleDeleteUpstream(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	upstreamID := r.URL.Query().Get("id")
	if upstreamID == "" {
		var req struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		upstreamID = req.ID
	}
	if upstreamID == "" {
		s.writeError(w, http.StatusBadRequest, "id parameter required")
		return
	}

	snap, err := s.state.DeleteUpstream(upstreamID, actor)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "DELETE_UPSTREAM", upstreamID, "SUCCESS", "Upstream removed", r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "upstream deleted and broadcast to fleet",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
	})
}

type publishRequest struct {
	Schema  state.ConfigSchema `json:"schema"`
	Comment string             `json:"comment"`
}

func (s *Server) handlePublishConfig(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req publishRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json schema: %v", err))
		return
	}

	snap, err := s.state.PublishNewConfig(req.Schema, actor, req.Comment)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Broadcast signed snapshot to all connected gateway data plane streams
	pushed := s.fleet.BroadcastSnapshot(snap)

	s.auth.RecordAudit(actor, string(role), "PUBLISH_CONFIG", fmt.Sprintf("v%d", snap.ConfigVersion), "SUCCESS", fmt.Sprintf("Pushed to %d nodes. Comment: %s", pushed, req.Comment), r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_published","version":%d,"nodes_notified":%d}`, snap.ConfigVersion, pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "configuration snapshot signed and published",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
		"signature_len":  len(snap.Ed25519Signature),
	})
}

type rollbackRequest struct {
	TargetVersion uint64 `json:"target_version"`
}

func (s *Server) handleRollbackConfig(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || role != auth.RoleAdmin {
		s.writeError(w, http.StatusForbidden, "requires admin role")
		return
	}

	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	snap, err := s.state.Rollback(req.TargetVersion, actor)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pushed := s.fleet.BroadcastSnapshot(snap)
	s.auth.RecordAudit(actor, string(role), "ROLLBACK_CONFIG", fmt.Sprintf("v%d->v%d", req.TargetVersion, snap.ConfigVersion), "SUCCESS", fmt.Sprintf("Restored snapshot version %d", req.TargetVersion), r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"config_rollback","restored_version":%d,"new_version":%d}`, req.TargetVersion, snap.ConfigVersion))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "rollback successful",
		"config_version": snap.ConfigVersion,
		"nodes_notified": pushed,
	})
}

func (s *Server) handleGetRevocations(w http.ResponseWriter, r *http.Request) {
	keys, tokens := s.state.GetRevocations()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"revoked_keys":   keys,
		"revoked_tokens": tokens,
	})
}

type revocationRequest struct {
	Keys   []string `json:"keys"`
	Tokens []string `json:"tokens"`
	Reason string   `json:"reason"`
}

func (s *Server) handleAddRevocation(w http.ResponseWriter, r *http.Request) {
	role, actor, err := s.auth.AuthenticateRequest(r)
	if err != nil || (role != auth.RoleAdmin && role != auth.RoleOperator) {
		s.writeError(w, http.StatusForbidden, "requires admin or operator role")
		return
	}

	var req revocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	rev := s.state.AddRevocation(req.Keys, req.Tokens)
	pushed := s.fleet.BroadcastRevocation(rev)

	s.auth.RecordAudit(actor, string(role), "REVOKE_CREDENTIALS", fmt.Sprintf("keys:%d,tokens:%d", len(req.Keys), len(req.Tokens)), "SUCCESS", req.Reason, r.RemoteAddr)
	s.broadcastSSE(fmt.Sprintf(`{"type":"revocation_issued","keys_count":%d,"tokens_count":%d,"nodes_notified":%d}`, len(req.Keys), len(req.Tokens), pushed))

	s.writeJSON(w, http.StatusOK, map[string]any{
		"message":        "credentials revoked and pushed to fleet",
		"nodes_notified": pushed,
	})
}

func (s *Server) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	usage := s.fleet.GetUsageSummary()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"usage_records": usage,
		"total":         len(usage),
	})
}

func (s *Server) handleGetAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	logs := s.auth.GetAuditLogs(limit)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"audit_logs": logs,
	})
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan string, 32)
	s.sseMu.Lock()
	s.sseClients[clientChan] = struct{}{}
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, clientChan)
		close(clientChan)
		s.sseMu.Unlock()
	}()

	// Send initial heartbeat
	_, _ = fmt.Fprintf(w, "event: init\ndata: {\"connected\":true,\"timestamp\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastSSE(data string) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()

	for ch := range s.sseClients {
		select {
		case ch <- data:
		default:
		}
	}
}

func (s *Server) handleStaticOrSPA(w http.ResponseWriter, r *http.Request) {
	// If path starts with /api/, it's a 404
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")

	// 1. Try disk uiDir first if provided
	if s.uiDir != "" {
		filePath := filepath.Join(s.uiDir, cleanPath)
		info, err := os.Stat(filePath)
		if err == nil && !info.IsDir() {
			http.ServeFile(w, r, filePath)
			return
		}
		// SPA fallback: serve index.html
		indexPath := filepath.Join(s.uiDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
	}

	// 2. Try embeddedFS if available
	if s.embeddedFS != nil {
		target := cleanPath
		if target == "" {
			target = "index.html"
		}
		f, err := s.embeddedFS.Open(target)
		if err == nil {
			_ = f.Close()
			http.FileServer(http.FS(s.embeddedFS)).ServeHTTP(w, r)
			return
		}
		// SPA fallback
		if indexFile, err := s.embeddedFS.Open("index.html"); err == nil {
			_ = indexFile.Close()
			r.URL.Path = "/"
			http.FileServer(http.FS(s.embeddedFS)).ServeHTTP(w, r)
			return
		}
	}

	// 3. Fallback welcome page if UI has not been built yet
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Torana Control Plane</title></head>
<body style="font-family:sans-serif;background:#0d1117;color:#c9d1d9;padding:40px;text-align:center;">
  <h1>Torana Control Plane</h1>
  <p>The Go gRPC & REST service is operational.</p>
  <p>To view the React dashboard, build the frontend with <code>npm run build</code> in <code>controlplane/ui</code>.</p>
</body>
</html>`)
}
