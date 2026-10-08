package agentauth

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Registry manages in-memory agent public keys and tenant verification keys.
type Registry struct {
	entries   sync.Map // string (AgentURI) -> registeredAgent
	tenantKey atomic.Pointer[ed25519.PublicKey]
}

type registeredAgent struct {
	entry     AgentRegistryEntry
	publicKey ed25519.PublicKey
}

// NewRegistry creates a new thread-safe agent and tenant public key registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// SetTenantPublicKey configures the tenant's Ed25519 public key used for root human user signatures.
func (r *Registry) SetTenantPublicKey(pubKey ed25519.PublicKey) {
	if len(pubKey) == ed25519.PublicKeySize {
		k := make(ed25519.PublicKey, ed25519.PublicKeySize)
		copy(k, pubKey)
		r.tenantKey.Store(&k)
	}
}

// GetTenantPublicKey retrieves the configured tenant public key.
func (r *Registry) GetTenantPublicKey() (ed25519.PublicKey, bool) {
	k := r.tenantKey.Load()
	if k == nil || len(*k) != ed25519.PublicKeySize {
		return nil, false
	}
	return *k, true
}

// RegisterAgent validates and registers an agent entry in the sync.Map.
func (r *Registry) RegisterAgent(entry AgentRegistryEntry) error {
	uri := strings.TrimSpace(entry.AgentURI)
	if uri == "" {
		return fmt.Errorf("empty agent_uri")
	}

	pubBytes, err := hex.DecodeString(strings.TrimSpace(entry.PublicKeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid ed25519 public_key_hex for agent %s (must be 64 hex chars): %w", uri, err)
	}

	pubKey := ed25519.PublicKey(pubBytes)
	r.entries.Store(uri, registeredAgent{
		entry:     entry,
		publicKey: pubKey,
	})
	return nil
}

// GetPublicKey retrieves the active Ed25519 public key for an agent URI.
// Returns false if agent is not registered or not active.
func (r *Registry) GetPublicKey(agentURI string) (ed25519.PublicKey, bool) {
	uri := strings.TrimSpace(agentURI)
	val, ok := r.entries.Load(uri)
	if !ok {
		return nil, false
	}
	reg, ok := val.(registeredAgent)
	if !ok || !reg.entry.Active {
		return nil, false
	}
	return reg.publicKey, true
}

// GetEntry retrieves the registered agent entry metadata.
func (r *Registry) GetEntry(agentURI string) (AgentRegistryEntry, bool) {
	uri := strings.TrimSpace(agentURI)
	val, ok := r.entries.Load(uri)
	if !ok {
		return AgentRegistryEntry{}, false
	}
	reg, ok := val.(registeredAgent)
	if !ok {
		return AgentRegistryEntry{}, false
	}
	return reg.entry, true
}

// UpdateFromSnapshot synchronizes the registry from a snapshot map of agents.
func (r *Registry) UpdateFromSnapshot(agents map[string]AgentRegistryEntry) {
	if agents == nil {
		return
	}
	for _, entry := range agents {
		_ = r.RegisterAgent(entry)
	}
}
