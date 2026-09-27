package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
)

// Signer handles Ed25519 cryptographic key generation, snapshot signing, and public key distribution.
type Signer struct {
	mu      sync.RWMutex
	pubKey  ed25519.PublicKey
	privKey ed25519.PrivateKey
	keyPath string
}

// NewSigner creates a new Ed25519 signer. If keyPath exists, it loads the key; otherwise it generates a new keypair.
func NewSigner(keyPath string) (*Signer, error) {
	s := &Signer{keyPath: keyPath}

	if keyPath != "" {
		if data, err := os.ReadFile(keyPath); err == nil && len(data) >= ed25519.PrivateKeySize {
			s.privKey = ed25519.PrivateKey(data[:ed25519.PrivateKeySize])
			s.pubKey = s.privKey.Public().(ed25519.PublicKey)
			return s, nil
		}
	}

	// Generate fresh keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 keypair: %w", err)
	}

	s.pubKey = pub
	s.privKey = priv

	if keyPath != "" {
		_ = os.WriteFile(keyPath, priv, 0600)
	}

	return s, nil
}

// SignSnapshot signs the canonical payload for a config version: "config_version:<ver>"
func (s *Signer) SignSnapshot(configVersion uint64) []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payload := []byte(fmt.Sprintf("config_version:%d", configVersion))
	return ed25519.Sign(s.privKey, payload)
}

// VerifySnapshot verifies a snapshot signature using the public key.
func (s *Signer) VerifySnapshot(configVersion uint64, signature []byte) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payload := []byte(fmt.Sprintf("config_version:%d", configVersion))
	return ed25519.Verify(s.pubKey, payload, signature)
}

// PublicKeyHex returns the hex-encoded Ed25519 public key.
func (s *Signer) PublicKeyHex() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return hex.EncodeToString(s.pubKey)
}

// PublicKeyBytes returns the raw Ed25519 public key bytes.
func (s *Signer) PublicKeyBytes() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buf := make([]byte, len(s.pubKey))
	copy(buf, s.pubKey)
	return buf
}
