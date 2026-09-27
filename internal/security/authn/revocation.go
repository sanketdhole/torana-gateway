package authn

import (
	"sync"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
)

// RevocationList manages revoked tokens and credentials with O(1) lock-optimized lookup and TTL eviction.
type RevocationList struct {
	mu          sync.RWMutex
	tokens      map[string]time.Time
	keys        map[string]time.Time
	subscribers map[chan struct{}]struct{}
}

// NewRevocationList creates an empty thread-safe revocation list.
func NewRevocationList() *RevocationList {
	return &RevocationList{
		tokens:      make(map[string]time.Time),
		keys:        make(map[string]time.Time),
		subscribers: make(map[chan struct{}]struct{}),
	}
}

func (r *RevocationList) notifySubscribersLocked() {
	for ch := range r.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// ApplyRevocation merges revocation instructions from control plane messages in O(N).
func (r *RevocationList) ApplyRevocation(rev *controlplanev1.Revocation) {
	if rev == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, tok := range rev.RevokedTokens {
		if tok != "" {
			r.tokens[tok] = time.Time{}
		}
	}
	for _, k := range rev.RevokedKeys {
		if k != "" {
			r.keys[k] = time.Time{}
		}
	}
	r.notifySubscribersLocked()
}

// RevokeToken registers an individual token identifier (e.g. jti, hash) as indefinitely revoked.
func (r *RevocationList) RevokeToken(tokenID string) {
	r.RevokeTokenWithExpiry(tokenID, time.Time{})
}

// RevokeTokenWithExpiry registers a token identifier as revoked until expiresAt.
func (r *RevocationList) RevokeTokenWithExpiry(tokenID string, expiresAt time.Time) {
	if tokenID == "" {
		return
	}
	r.mu.Lock()
	r.tokens[tokenID] = expiresAt
	r.notifySubscribersLocked()
	r.mu.Unlock()
}

// RevokeKey registers an API key identifier, prefix, or hash as indefinitely revoked.
func (r *RevocationList) RevokeKey(keyID string) {
	r.RevokeKeyWithExpiry(keyID, time.Time{})
}

// RevokeKeyWithExpiry registers an API key identifier as revoked until expiresAt.
func (r *RevocationList) RevokeKeyWithExpiry(keyID string, expiresAt time.Time) {
	if keyID == "" {
		return
	}
	r.mu.Lock()
	r.keys[keyID] = expiresAt
	r.notifySubscribersLocked()
	r.mu.Unlock()
}

// IsTokenRevoked checks if a token ID or hash is revoked in O(1) time.
func (r *RevocationList) IsTokenRevoked(tokenID string) bool {
	if tokenID == "" {
		return false
	}
	r.mu.RLock()
	exp, revoked := r.tokens[tokenID]
	r.mu.RUnlock()
	if !revoked {
		return false
	}
	if !exp.IsZero() && time.Now().After(exp) {
		return false
	}
	return true
}

// IsKeyRevoked checks if an API key identifier or hash is revoked in O(1) time.
func (r *RevocationList) IsKeyRevoked(keyID string) bool {
	if keyID == "" {
		return false
	}
	r.mu.RLock()
	exp, revoked := r.keys[keyID]
	r.mu.RUnlock()
	if !revoked {
		return false
	}
	if !exp.IsZero() && time.Now().After(exp) {
		return false
	}
	return true
}

// IsRevoked checks whether the given ID matches any non-expired revoked token or key in O(1).
func (r *RevocationList) IsRevoked(id string) bool {
	if id == "" {
		return false
	}
	now := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	if exp, ok := r.tokens[id]; ok {
		if exp.IsZero() || now.Before(exp) {
			return true
		}
	}
	if exp, ok := r.keys[id]; ok {
		if exp.IsZero() || now.Before(exp) {
			return true
		}
	}
	return false
}

// Prune purges expired token and key entries from memory (S10).
func (r *RevocationList) Prune(now time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var pruned int
	for tok, exp := range r.tokens {
		if !exp.IsZero() && now.After(exp) {
			delete(r.tokens, tok)
			pruned++
		}
	}
	for k, exp := range r.keys {
		if !exp.IsZero() && now.After(exp) {
			delete(r.keys, k)
			pruned++
		}
	}
	return pruned
}

// Count returns the number of active, non-expired revoked tokens and keys.
func (r *RevocationList) Count() (tokens int, keys int) {
	now := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, exp := range r.tokens {
		if exp.IsZero() || now.Before(exp) {
			tokens++
		}
	}
	for _, exp := range r.keys {
		if exp.IsZero() || now.Before(exp) {
			keys++
		}
	}
	return tokens, keys
}

// Subscribe registers a listener channel notified on any revocation update.
// Returns the event channel and an unsubscribe cleanup function.
func (r *RevocationList) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	if r.subscribers == nil {
		r.subscribers = make(map[chan struct{}]struct{})
	}
	r.subscribers[ch] = struct{}{}
	r.mu.Unlock()

	unsubscribe := func() {
		r.mu.Lock()
		delete(r.subscribers, ch)
		r.mu.Unlock()
	}
	return ch, unsubscribe
}


