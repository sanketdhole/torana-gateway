package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"github.com/phaselume/torana/controlplane/pkg/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrVersionNotFound = errors.New("configuration version not found in history")
)

// ConfigSchema represents the persisted schema.
type ConfigSchema struct {
	Version           uint64                             `json:"version"`
	UpdatedAt         time.Time                          `json:"updated_at"`
	Routes            []*controlplanev1.Route            `json:"routes"`
	Upstreams         []*controlplanev1.Upstream         `json:"upstreams"`
	Policies          []*controlplanev1.Policy           `json:"policies"`
	TokenBudgets      []*controlplanev1.TokenBudget      `json:"token_budgets,omitempty"`
	AuthnProviders    []*controlplanev1.AuthnProvider    `json:"authn_providers,omitempty"`
	PluginAssignments []*controlplanev1.PluginAssignment `json:"plugin_assignments,omitempty"`
	RevokedKeys       []string                           `json:"revoked_keys,omitempty"`
	RevokedTokens     []string                           `json:"revoked_tokens,omitempty"`
}

// ConfigHistoryRecord holds past snapshots for rollback capability.
type ConfigHistoryRecord struct {
	Version   uint64    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Author    string    `json:"author"`
	Comment   string    `json:"comment"`
}

// Store manages dynamic state, versioning, cryptographic signing, and persistence.
type Store struct {
	mu             sync.RWMutex
	configVersion  atomic.Uint64
	currentSnap    *controlplanev1.Snapshot
	schema         ConfigSchema
	history        map[uint64]*controlplanev1.Snapshot
	historyMeta    []ConfigHistoryRecord
	revokedKeys    map[string]time.Time
	revokedTokens  map[string]time.Time
	signer         *crypto.Signer
	storagePath    string
	logger         *slog.Logger
	maxHistory     int
}

// NewStore initializes the state store, loading existing config or establishing defaults.
func NewStore(storagePath string, signer *crypto.Signer, logger *slog.Logger) (*Store, error) {
	s := &Store{
		history:       make(map[uint64]*controlplanev1.Snapshot),
		historyMeta:   make([]ConfigHistoryRecord, 0, 50),
		revokedKeys:   make(map[string]time.Time),
		revokedTokens: make(map[string]time.Time),
		signer:        signer,
		storagePath:   storagePath,
		logger:        logger,
		maxHistory:    50,
	}

	if storagePath != "" {
		if err := s.loadFromFile(); err != nil {
			logger.Warn("could not load existing config file, initializing defaults", "path", storagePath, "err", err)
			s.initDefaultConfig()
		}
	} else {
		s.initDefaultConfig()
	}

	return s, nil
}

func (s *Store) initDefaultConfig() {
	s.schema = ConfigSchema{
		Version:   1,
		UpdatedAt: time.Now().UTC(),
		Routes: []*controlplanev1.Route{
			{
				Id:         "route-openai-llm",
				Path:       "/v1/chat/completions",
				PathPrefix: false,
				Method:     "POST",
				UpstreamId: "upstream-llm-primary",
				PolicyIds:  []string{"policy-auth-jwt", "policy-cel-guard"},
				TimeoutMs:  60000,
			},
			{
				Id:         "route-mcp-tools",
				Path:       "/mcp",
				PathPrefix: true,
				Method:     "POST",
				UpstreamId: "upstream-mcp-server",
				PolicyIds:  []string{"policy-auth-jwt"},
				TimeoutMs:  30000,
			},
		},
		Upstreams: []*controlplanev1.Upstream{
			{
				Id:        "upstream-llm-primary",
				Protocol:  "llm",
				Endpoints: []string{"https://api.openai.com"},
				TimeoutMs: 60000,
				MaxConns:  100,
			},
			{
				Id:        "upstream-mcp-server",
				Protocol:  "mcp",
				Endpoints: []string{"http://127.0.0.1:8090"},
				TimeoutMs: 30000,
				MaxConns:  50,
			},
		},
		Policies: []*controlplanev1.Policy{
			{
				Id:     "policy-auth-jwt",
				Name:   "Enterprise JWT Authentication",
				Type:   "authn",
				Action: "ALLOW",
				Parameters: map[string]string{
					"provider": "default-jwt",
				},
			},
			{
				Id:            "policy-cel-guard",
				Name:          "Prompt Security & Data Exfiltration Guard",
				Type:          "cel",
				CelExpression: "request.auth.tenant_id != ''",
				Action:        "ALLOW",
			},
		},
		TokenBudgets: []*controlplanev1.TokenBudget{
			{
				TenantId:                 "tenant-engineering",
				ModelId:                  "gpt-4o",
				MaxPromptTokensPerMin:    100000,
				MaxCompletionTokensPerMin: 50000,
				MaxCostPerDay:            250.0,
			},
		},
	}

	s.configVersion.Store(1)
	s.buildAndSignSnapshot("System Initial Bootstrap")
	_ = s.persistToFile()
}

func (s *Store) buildAndSignSnapshot(comment string) {
	ver := s.configVersion.Load()

	snap := &controlplanev1.Snapshot{
		ConfigVersion:     ver,
		CreatedAt:         timestamppb.Now(),
		Routes:            s.schema.Routes,
		Upstreams:         s.schema.Upstreams,
		Policies:          s.schema.Policies,
		TokenBudgets:      s.schema.TokenBudgets,
		AuthnProviders:    s.schema.AuthnProviders,
		PluginAssignments: s.schema.PluginAssignments,
	}

	if s.signer != nil {
		sig := s.signer.SignSnapshot(ver)
		snap.Ed25519Signature = sig
	}

	s.currentSnap = snap
	s.history[ver] = snap

	record := ConfigHistoryRecord{
		Version:   ver,
		Timestamp: time.Now().UTC(),
		Author:    "admin",
		Comment:   comment,
	}
	s.historyMeta = append(s.historyMeta, record)
	if len(s.historyMeta) > s.maxHistory {
		delete(s.history, s.historyMeta[0].Version)
		s.historyMeta = s.historyMeta[1:]
	}
}

// CurrentSnapshot returns the active signed configuration snapshot.
func (s *Store) CurrentSnapshot() *controlplanev1.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentSnap
}

// ConfigVersion returns the current monotonic configuration version.
func (s *Store) ConfigVersion() uint64 {
	return s.configVersion.Load()
}

// GetSchema returns the current JSON schema representation.
func (s *Store) GetSchema() ConfigSchema {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.schema
}

// GetHistory returns snapshot version history metadata.
func (s *Store) GetHistory() []ConfigHistoryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]ConfigHistoryRecord, len(s.historyMeta))
	copy(res, s.historyMeta)
	return res
}

// PublishNewConfig updates configuration schema, increments version, and signs the snapshot.
func (s *Store) PublishNewConfig(schema ConfigSchema, author, comment string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newVer := s.configVersion.Add(1)
	schema.Version = newVer
	schema.UpdatedAt = time.Now().UTC()

	s.schema = schema
	s.buildAndSignSnapshot(comment)

	if err := s.persistToFile(); err != nil {
		s.logger.Error("failed to persist published config to disk", "error", err)
	}

	s.logger.Info("published new configuration snapshot",
		"version", newVer,
		"routes", len(schema.Routes),
		"upstreams", len(schema.Upstreams),
		"policies", len(schema.Policies),
		"author", author,
	)

	return s.currentSnap, nil
}

// UpsertRoute adds or updates a route, increments version, and signs the snapshot.
func (s *Store) UpsertRoute(r *controlplanev1.Route, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, existing := range s.schema.Routes {
		if existing.Id == r.Id {
			s.schema.Routes[i] = r
			found = true
			break
		}
	}
	if !found {
		s.schema.Routes = append(s.schema.Routes, r)
	}

	newVer := s.configVersion.Add(1)
	s.schema.Version = newVer
	s.schema.UpdatedAt = time.Now().UTC()
	s.buildAndSignSnapshot(fmt.Sprintf("Update route %s", r.Id))
	_ = s.persistToFile()

	return s.currentSnap, nil
}

// DeleteRoute removes a route by ID.
func (s *Store) DeleteRoute(routeID string, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]*controlplanev1.Route, 0, len(s.schema.Routes))
	for _, r := range s.schema.Routes {
		if r.Id != routeID {
			filtered = append(filtered, r)
		}
	}
	s.schema.Routes = filtered

	newVer := s.configVersion.Add(1)
	s.schema.Version = newVer
	s.schema.UpdatedAt = time.Now().UTC()
	s.buildAndSignSnapshot(fmt.Sprintf("Delete route %s", routeID))
	_ = s.persistToFile()

	return s.currentSnap, nil
}

// UpsertPolicy adds or updates a policy, increments version, and signs the snapshot.
func (s *Store) UpsertPolicy(p *controlplanev1.Policy, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, existing := range s.schema.Policies {
		if existing.Id == p.Id {
			s.schema.Policies[i] = p
			found = true
			break
		}
	}
	if !found {
		s.schema.Policies = append(s.schema.Policies, p)
	}

	newVer := s.configVersion.Add(1)
	s.schema.Version = newVer
	s.schema.UpdatedAt = time.Now().UTC()
	s.buildAndSignSnapshot(fmt.Sprintf("Update policy %s (%s)", p.Name, p.Id))
	_ = s.persistToFile()

	return s.currentSnap, nil
}

// DeletePolicy removes a policy by ID.
func (s *Store) DeletePolicy(policyID string, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]*controlplanev1.Policy, 0, len(s.schema.Policies))
	for _, p := range s.schema.Policies {
		if p.Id != policyID {
			filtered = append(filtered, p)
		}
	}
	s.schema.Policies = filtered

	newVer := s.configVersion.Add(1)
	s.schema.Version = newVer
	s.schema.UpdatedAt = time.Now().UTC()
	s.buildAndSignSnapshot(fmt.Sprintf("Delete policy %s", policyID))
	_ = s.persistToFile()

	return s.currentSnap, nil
}

// UpsertUpstream adds or updates an upstream cluster.
func (s *Store) UpsertUpstream(u *controlplanev1.Upstream, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i, existing := range s.schema.Upstreams {
		if existing.Id == u.Id {
			s.schema.Upstreams[i] = u
			found = true
			break
		}
	}
	if !found {
		s.schema.Upstreams = append(s.schema.Upstreams, u)
	}

	newVer := s.configVersion.Add(1)
	s.schema.Version = newVer
	s.schema.UpdatedAt = time.Now().UTC()
	s.buildAndSignSnapshot(fmt.Sprintf("Update upstream %s", u.Id))
	_ = s.persistToFile()

	return s.currentSnap, nil
}

// DeleteUpstream removes an upstream by ID.
func (s *Store) DeleteUpstream(upstreamID string, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]*controlplanev1.Upstream, 0, len(s.schema.Upstreams))
	for _, u := range s.schema.Upstreams {
		if u.Id != upstreamID {
			filtered = append(filtered, u)
		}
	}
	s.schema.Upstreams = filtered

	newVer := s.configVersion.Add(1)
	s.schema.Version = newVer
	s.schema.UpdatedAt = time.Now().UTC()
	s.buildAndSignSnapshot(fmt.Sprintf("Delete upstream %s", upstreamID))
	_ = s.persistToFile()

	return s.currentSnap, nil
}

// Rollback restores a previous snapshot version from history as a new monotonically incremented version.
func (s *Store) Rollback(targetVersion uint64, author string) (*controlplanev1.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	targetSnap, exists := s.history[targetVersion]
	if !exists {
		return nil, fmt.Errorf("%w: version %d", ErrVersionNotFound, targetVersion)
	}

	newVer := s.configVersion.Add(1)
	s.schema = ConfigSchema{
		Version:           newVer,
		UpdatedAt:         time.Now().UTC(),
		Routes:            targetSnap.Routes,
		Upstreams:         targetSnap.Upstreams,
		Policies:          targetSnap.Policies,
		TokenBudgets:      targetSnap.TokenBudgets,
		AuthnProviders:    targetSnap.AuthnProviders,
		PluginAssignments: targetSnap.PluginAssignments,
	}

	comment := fmt.Sprintf("Rollback to snapshot version %d", targetVersion)
	s.buildAndSignSnapshot(comment)
	_ = s.persistToFile()

	s.logger.Info("performed configuration rollback",
		"restored_from_version", targetVersion,
		"new_version", newVer,
		"author", author,
	)

	return s.currentSnap, nil
}

// AddRevocation registers revoked tokens or keys.
func (s *Store) AddRevocation(keys, tokens []string) *controlplanev1.Revocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, k := range keys {
		if k != "" {
			s.revokedKeys[k] = now
		}
	}
	for _, t := range tokens {
		if t != "" {
			s.revokedTokens[t] = now
		}
	}

	return &controlplanev1.Revocation{
		RevokedKeys:   keys,
		RevokedTokens: tokens,
	}
}

// GetRevocations returns the list of all currently active revoked items.
func (s *Store) GetRevocations() (keys []string, tokens []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for k := range s.revokedKeys {
		keys = append(keys, k)
	}
	for t := range s.revokedTokens {
		tokens = append(tokens, t)
	}
	return keys, tokens
}

func (s *Store) persistToFile() error {
	if s.storagePath == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.schema, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.storagePath, data, 0644)
}

func (s *Store) loadFromFile() error {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return err
	}

	var schema ConfigSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return err
	}

	if schema.Version == 0 {
		schema.Version = 1
	}

	s.schema = schema
	s.configVersion.Store(schema.Version)
	s.buildAndSignSnapshot("Loaded from storage")
	return nil
}
