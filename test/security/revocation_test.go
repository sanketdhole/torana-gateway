package security_test

import (
	"fmt"
	"testing"
	"time"

	controlplanev1 "github.com/phaselume/torana/api/proto/controlplane/v1"
	"github.com/phaselume/torana/internal/security/authn"
)

func TestRevocationList_ApplyFromProtoMessage(t *testing.T) {
	revList := authn.NewRevocationList()

	msg := &controlplanev1.Revocation{
		RevokedTokens: []string{"tok-101", "tok-102", "tok-103"},
		RevokedKeys:   []string{"key-pref-1", "key-pref-2"},
	}

	revList.ApplyRevocation(msg)

	tokCount, keyCount := revList.Count()
	if tokCount != 3 || keyCount != 2 {
		t.Fatalf("expected 3 tokens and 2 keys, got %d tokens, %d keys", tokCount, keyCount)
	}

	if !revList.IsTokenRevoked("tok-102") {
		t.Errorf("expected tok-102 to be revoked")
	}
	if revList.IsTokenRevoked("tok-nonexistent") {
		t.Errorf("expected tok-nonexistent NOT to be revoked")
	}

	if !revList.IsKeyRevoked("key-pref-1") {
		t.Errorf("expected key-pref-1 to be revoked")
	}
	if revList.IsKeyRevoked("key-pref-nonexistent") {
		t.Errorf("expected key-pref-nonexistent NOT to be revoked")
	}

	if !revList.IsRevoked("tok-103") {
		t.Errorf("expected IsRevoked(tok-103) == true")
	}
	if !revList.IsRevoked("key-pref-2") {
		t.Errorf("expected IsRevoked(key-pref-2) == true")
	}
	if revList.IsRevoked("random-clean-id") {
		t.Errorf("expected IsRevoked(random-clean-id) == false")
	}
}

func TestRevocationList_TTLExpirationAndPruning(t *testing.T) {
	revList := authn.NewRevocationList()

	// Revoke a token with expiry in the past
	revList.RevokeTokenWithExpiry("expired-tok", time.Now().Add(-10*time.Minute))
	// Revoke a key with expiry in the future
	revList.RevokeKeyWithExpiry("active-key", time.Now().Add(10*time.Minute))
	// Revoke token with no expiry (indefinite)
	revList.RevokeToken("indefinite-tok")

	// Expired token should not be considered revoked
	if revList.IsTokenRevoked("expired-tok") {
		t.Errorf("expected expired-tok NOT to be revoked")
	}
	if revList.IsRevoked("expired-tok") {
		t.Errorf("expected IsRevoked(expired-tok) to be false")
	}

	// Active key should be considered revoked
	if !revList.IsKeyRevoked("active-key") {
		t.Errorf("expected active-key to be revoked")
	}
	if !revList.IsTokenRevoked("indefinite-tok") {
		t.Errorf("expected indefinite-tok to be revoked")
	}

	// Prune should remove expired-tok from internal map
	pruned := revList.Prune(time.Now())
	if pruned != 1 {
		t.Errorf("expected 1 entry pruned, got %d", pruned)
	}

	tokCount, keyCount := revList.Count()
	if tokCount != 1 || keyCount != 1 {
		t.Errorf("expected 1 token and 1 key remaining, got %d tokens, %d keys", tokCount, keyCount)
	}
}

func BenchmarkRevocationList_O1Lookup(b *testing.B) {
	revList := authn.NewRevocationList()

	// Seed 100,000 revoked entries
	const numEntries = 100_000
	tokens := make([]string, numEntries)
	for i := 0; i < numEntries; i++ {
		tok := fmt.Sprintf("revoked-token-uuid-%08d", i)
		tokens[i] = tok
		revList.RevokeToken(tok)
	}

	target := tokens[numEntries/2]

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !revList.IsTokenRevoked(target) {
			b.Fatal("expected target to be revoked")
		}
	}
}
