package agentauth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Standard error definitions for delegation token processing.
var (
	ErrInvalidPrincipalURI = errors.New("invalid principal uri")
	ErrEmptyToken           = errors.New("empty delegation token")
	ErrInvalidTokenEncoding = errors.New("invalid token encoding: expected JSON or base64 JSON")
	ErrInvalidTokenPayload  = errors.New("invalid delegation token payload")
)

// PrincipalType defines the strictly supported entity classifications in Torana.
type PrincipalType string

const (
	PrincipalTypeUser   PrincipalType = "user"
	PrincipalTypeAgent  PrincipalType = "agent"
	PrincipalTypeTool   PrincipalType = "tool"
	PrincipalTypeSystem PrincipalType = "system"
)

// Principal represents a parsed universal Torana principal URI:
// torana://<org_id>/<namespace>/<principal_type>/<principal_id>
type Principal struct {
	URI       string        `json:"uri"`
	OrgID     string        `json:"org_id"`
	Namespace string        `json:"namespace"`
	Type      PrincipalType `json:"type"`
	ID        string        `json:"id"`
}

// ParsePrincipalURI validates and decomposes a universal principal URI.
// Format: torana://<org_id>/<namespace>/<principal_type>/<principal_id>
func ParsePrincipalURI(raw string) (*Principal, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "torana://") {
		return nil, fmt.Errorf("%w: must begin with 'torana://'", ErrInvalidPrincipalURI)
	}

	path := strings.TrimPrefix(raw, "torana://")
	parts := strings.Split(path, "/")
	if len(parts) != 4 {
		return nil, fmt.Errorf("%w: expected 4 segments <org_id>/<namespace>/<principal_type>/<principal_id>, got %d", ErrInvalidPrincipalURI, len(parts))
	}

	orgID := strings.TrimSpace(parts[0])
	namespace := strings.TrimSpace(parts[1])
	pTypeStr := strings.TrimSpace(parts[2])
	id := strings.TrimSpace(parts[3])

	if orgID == "" || namespace == "" || pTypeStr == "" || id == "" {
		return nil, fmt.Errorf("%w: segments must not be empty", ErrInvalidPrincipalURI)
	}

	pType := PrincipalType(strings.ToLower(pTypeStr))
	switch pType {
	case PrincipalTypeUser, PrincipalTypeAgent, PrincipalTypeTool, PrincipalTypeSystem:
	default:
		return nil, fmt.Errorf("%w: unrecognized principal_type %q (must be user, agent, tool, or system)", ErrInvalidPrincipalURI, pTypeStr)
	}

	return &Principal{
		URI:       raw,
		OrgID:     orgID,
		Namespace: namespace,
		Type:      pType,
		ID:        id,
	}, nil
}

// AgentRegistryEntry configures an AI agent's public key and risk posture.
type AgentRegistryEntry struct {
	AgentURI      string   `json:"agent_uri"`
	PublicKeyHex  string   `json:"public_key_hex"` // Ed25519 public key (64 hex chars)
	RiskTier      string   `json:"risk_tier"`      // LOW, MEDIUM, HIGH, CRITICAL
	AllowedScopes []string `json:"allowed_scopes"`
	Active        bool     `json:"active"`
}

// DelegationRoot represents the initial delegation origin signed by the root principal.
type DelegationRoot struct {
	Principal     string   `json:"principal"`      // e.g. "torana://org/ns/user/usr_1"
	IssuedAt      int64    `json:"issued_at"`      // Unix timestamp seconds
	ExpiresAt     int64    `json:"expires_at"`     // Unix timestamp seconds
	InitialScopes []string `json:"initial_scopes"` // e.g. ["analytics:*", "db:read"]
	SignatureHex  string   `json:"signature"`      // Ed25519 signature of CanonicalRootPayload
}

// DelegationHop represents a downstream attenuation hop in an agent delegation chain.
type DelegationHop struct {
	HopIndex         int               `json:"hop_index"`                 // 1-indexed
	DelegatorURI     string            `json:"delegator_uri"`             // Must match previous delegatee or root
	DelegateeURI     string            `json:"delegatee_uri"`             // Next agent in chain
	AttenuatedScopes []string          `json:"attenuated_scopes"`         // MUST be subset of delegator scopes
	Constraints      map[string]string `json:"constraints,omitempty"`    // e.g. "max_downstream_hops": "2"
	IssuedAt         int64             `json:"issued_at"`
	ExpiresAt        int64             `json:"expires_at"`
	SignatureHex     string            `json:"signature"`                 // Ed25519 signature of CanonicalHopPayload
}

// ToranaDelegationToken (TDT) represents the multi-hop agent delegation wire payload.
type ToranaDelegationToken struct {
	Version string          `json:"version"` // "1.0"
	Root    DelegationRoot  `json:"root"`
	Hops    []DelegationHop `json:"hops"`
}

// ChainHopContext contains individual hop details for CEL evaluation.
type ChainHopContext struct {
	Delegator string   `json:"delegator"`
	Delegatee string   `json:"delegatee"`
	Scopes    []string `json:"scopes"`
}

// ChainContext is the verified delegation context exposed to CEL policies and upstream headers.
type ChainContext struct {
	RootPrincipal   string            `json:"root_principal"`
	RootType        string            `json:"root_type"`
	Depth           int               `json:"depth"`
	EffectiveScopes []string          `json:"effective_scopes"`
	CallerURI       string            `json:"caller_uri"`
	Hops            []ChainHopContext `json:"hops"`
}

// VerificationResult contains the result of delegation verification.
type VerificationResult struct {
	Valid           bool          `json:"valid"`
	Reason          string        `json:"reason,omitempty"`
	StatusCode      int           `json:"status_code,omitempty"` // 401 or 403
	ChainContext    *ChainContext `json:"chain_context,omitempty"`
	RootPrincipal   string        `json:"root_principal,omitempty"`
	CallerPrincipal string        `json:"caller_principal,omitempty"`
	ChainDepth      int           `json:"chain_depth,omitempty"`
	EffectiveScopes []string      `json:"effective_scopes,omitempty"`
}

// ParseDelegationToken deserializes a raw or base64-encoded JSON Torana Delegation Token.
func ParseDelegationToken(raw string) (*ToranaDelegationToken, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrEmptyToken
	}

	var jsonBytes []byte
	if strings.HasPrefix(trimmed, "{") {
		jsonBytes = []byte(trimmed)
	} else {
		// Attempt standard Base64 decode, then URL-safe Base64 decode
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(trimmed)
			if err != nil {
				decoded, err = base64.RawURLEncoding.DecodeString(trimmed)
				if err != nil {
					return nil, fmt.Errorf("%w: %v", ErrInvalidTokenEncoding, err)
				}
			}
		}
		jsonBytes = decoded
	}

	var token ToranaDelegationToken
	if err := json.Unmarshal(jsonBytes, &token); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTokenPayload, err)
	}

	if token.Version == "" {
		token.Version = "1.0"
	}

	return &token, nil
}
