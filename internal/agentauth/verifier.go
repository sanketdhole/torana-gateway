package agentauth

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RevocationChecker checks whether a principal URI or token is revoked.
type RevocationChecker interface {
	IsTokenRevoked(id string) bool
	IsKeyRevoked(id string) bool
}

// Verifier enforces Zero-Trust cryptographic and policy invariants on Torana Delegation Tokens.
type Verifier struct {
	registry   *Registry
	revocations RevocationChecker
	nowFunc    func() time.Time
}

// NewVerifier creates a new delegation token verifier.
func NewVerifier(registry *Registry, revocations RevocationChecker) *Verifier {
	return &Verifier{
		registry:    registry,
		revocations: revocations,
		nowFunc:     time.Now,
	}
}

// SetNowFunc overrides the clock for deterministic testing.
func (v *Verifier) SetNowFunc(f func() time.Time) {
	v.nowFunc = f
}

// Verify enforces all delegation invariants and returns a VerificationResult.
func (v *Verifier) Verify(ctx context.Context, token *ToranaDelegationToken) *VerificationResult {
	if token == nil {
		return &VerificationResult{
			Valid:      false,
			StatusCode: http.StatusUnauthorized,
			Reason:     "nil_delegation_token",
		}
	}

	now := v.nowFunc().Unix()

	// 1. Root Expiry
	if token.Root.ExpiresAt <= now {
		return &VerificationResult{
			Valid:      false,
			StatusCode: http.StatusUnauthorized,
			Reason:     "root_token_expired",
		}
	}

	// 2. Parse Root Principal URI
	rootPrincipal, err := ParsePrincipalURI(token.Root.Principal)
	if err != nil {
		return &VerificationResult{
			Valid:      false,
			StatusCode: http.StatusBadRequest,
			Reason:     fmt.Sprintf("invalid_root_principal_uri: %v", err),
		}
	}

	// 3. Revocation Check on Root Principal
	if v.isRevoked(token.Root.Principal) {
		return &VerificationResult{
			Valid:      false,
			StatusCode: http.StatusForbidden,
			Reason:     fmt.Sprintf("principal_revoked: %s", token.Root.Principal),
		}
	}

	// 4. Verify Root Signature
	var rootPubKey ed25519.PublicKey
	if rootPrincipal.Type == PrincipalTypeUser {
		if v.registry == nil {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     "tenant_public_key_not_configured",
			}
		}
		key, ok := v.registry.GetTenantPublicKey()
		if !ok {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     "tenant_public_key_missing",
			}
		}
		rootPubKey = key
	} else {
		// Agent, System, or Tool root initiator
		if v.registry == nil {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     "agent_registry_not_configured",
			}
		}
		key, ok := v.registry.GetPublicKey(token.Root.Principal)
		if !ok {
			// Fallback check on tenant key if root registered by tenant
			tKey, tOk := v.registry.GetTenantPublicKey()
			if tOk {
				key = tKey
				ok = true
			}
		}
		if !ok {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     fmt.Sprintf("unregistered_or_inactive_root_principal: %s", token.Root.Principal),
			}
		}
		rootPubKey = key
	}

	if !VerifyRootSignature(rootPubKey, &token.Root) {
		return &VerificationResult{
			Valid:      false,
			StatusCode: http.StatusUnauthorized,
			Reason:     "invalid_root_signature",
		}
	}

	// 5. Verify Hops Chain
	prevScopes := token.Root.InitialScopes
	prevExpiresAt := token.Root.ExpiresAt
	prevSigHex := token.Root.SignatureHex
	prevDelegatee := token.Root.Principal

	chainHops := make([]ChainHopContext, 0, len(token.Hops))

	for i, hop := range token.Hops {
		expectedIndex := i + 1
		if hop.HopIndex != expectedIndex {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusBadRequest,
				Reason:     fmt.Sprintf("invalid_hop_index: expected %d, got %d", expectedIndex, hop.HopIndex),
			}
		}

		// Chain Continuity
		if strings.TrimSpace(hop.DelegatorURI) != strings.TrimSpace(prevDelegatee) {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusBadRequest,
				Reason:     fmt.Sprintf("chain_continuity_broken: hop %d delegator %q does not match preceding %q", expectedIndex, hop.DelegatorURI, prevDelegatee),
			}
		}

		// Parse Delegator & Delegatee URIs
		delegatorPrincipal, err := ParsePrincipalURI(hop.DelegatorURI)
		if err != nil {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusBadRequest,
				Reason:     fmt.Sprintf("invalid_delegator_uri at hop %d: %v", expectedIndex, err),
			}
		}
		_, err = ParsePrincipalURI(hop.DelegateeURI)
		if err != nil {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusBadRequest,
				Reason:     fmt.Sprintf("invalid_delegatee_uri at hop %d: %v", expectedIndex, err),
			}
		}

		// Revocation Check on Intermediate/Final Delegatee
		if v.isRevoked(hop.DelegateeURI) {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusForbidden,
				Reason:     fmt.Sprintf("principal_revoked: %s", hop.DelegateeURI),
			}
		}

		// Hop Expiration & TTL Decay
		if hop.ExpiresAt <= now {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     fmt.Sprintf("hop_expired: hop %d expired at %d", expectedIndex, hop.ExpiresAt),
			}
		}
		if hop.ExpiresAt > prevExpiresAt {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusBadRequest,
				Reason:     fmt.Sprintf("ttl_decay_violation: hop %d expires at %d after parent %d", expectedIndex, hop.ExpiresAt, prevExpiresAt),
			}
		}

		// Monotonic Scope Attenuation Check
		if ok, offending := ScopesCover(prevScopes, hop.AttenuatedScopes); !ok {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusForbidden,
				Reason:     fmt.Sprintf("scope_escalation_detected: hop %d scope %q not covered by parent scopes", expectedIndex, offending),
			}
		}

		// Hop Depth Constraints (max_downstream_hops)
		if hop.Constraints != nil {
			if maxHopsStr, exists := hop.Constraints["max_downstream_hops"]; exists {
				maxHops, err := strconv.Atoi(strings.TrimSpace(maxHopsStr))
				if err == nil && maxHops >= 0 {
					remainingHops := len(token.Hops) - 1 - i
					if remainingHops > maxHops {
						return &VerificationResult{
							Valid:      false,
							StatusCode: http.StatusForbidden,
							Reason:     fmt.Sprintf("max_downstream_hops_exceeded: hop %d allows at most %d downstream hops, got %d", expectedIndex, maxHops, remainingHops),
						}
					}
				}
			}
		}

		// Delegator Public Key Resolution
		var hopPubKey ed25519.PublicKey
		if delegatorPrincipal.Type == PrincipalTypeUser {
			if v.registry != nil {
				if tKey, ok := v.registry.GetTenantPublicKey(); ok {
					hopPubKey = tKey
				}
			}
		} else {
			if v.registry != nil {
				if aKey, ok := v.registry.GetPublicKey(hop.DelegatorURI); ok {
					hopPubKey = aKey
				}
			}
		}

		if len(hopPubKey) != ed25519.PublicKeySize {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     fmt.Sprintf("unregistered_or_inactive_delegator: %s", hop.DelegatorURI),
			}
		}

		// Cryptographic Binding to Preceding Hop (prevSigHex)
		if !VerifyHopSignature(hopPubKey, &hop, prevSigHex) {
			return &VerificationResult{
				Valid:      false,
				StatusCode: http.StatusUnauthorized,
				Reason:     fmt.Sprintf("invalid_hop_signature: hop %d", expectedIndex),
			}
		}

		chainHops = append(chainHops, ChainHopContext{
			Delegator: hop.DelegatorURI,
			Delegatee: hop.DelegateeURI,
			Scopes:    hop.AttenuatedScopes,
		})

		prevScopes = hop.AttenuatedScopes
		prevExpiresAt = hop.ExpiresAt
		prevSigHex = hop.SignatureHex
		prevDelegatee = hop.DelegateeURI
	}

	// 6. Construct Verified Chain Context
	effectiveScopes := token.Root.InitialScopes
	callerURI := token.Root.Principal
	if len(token.Hops) > 0 {
		effectiveScopes = token.Hops[len(token.Hops)-1].AttenuatedScopes
		callerURI = token.Hops[len(token.Hops)-1].DelegateeURI
	}

	chainCtx := &ChainContext{
		RootPrincipal:   token.Root.Principal,
		RootType:        string(rootPrincipal.Type),
		Depth:           len(token.Hops),
		EffectiveScopes: effectiveScopes,
		CallerURI:       callerURI,
		Hops:            chainHops,
	}

	return &VerificationResult{
		Valid:           true,
		StatusCode:      http.StatusOK,
		ChainContext:    chainCtx,
		RootPrincipal:   token.Root.Principal,
		CallerPrincipal: callerURI,
		ChainDepth:      len(token.Hops),
		EffectiveScopes: effectiveScopes,
	}
}

func (v *Verifier) isRevoked(principalURI string) bool {
	if v.revocations == nil || principalURI == "" {
		return false
	}
	return v.revocations.IsTokenRevoked(principalURI) || v.revocations.IsKeyRevoked(principalURI)
}
