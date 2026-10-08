package agentauth

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrInvalidPrivateKey = errors.New("invalid ed25519 private key")
	ErrInvalidPublicKey  = errors.New("invalid ed25519 public key")
	ErrSignatureFailed   = errors.New("signature creation failed")
)

// CanonicalRootPayload generates the deterministic UTF-8 bytes to sign/verify for a DelegationRoot:
// root:{principal}:{issued_at}:{expires_at}:{sorted_comma_separated_scopes}
func CanonicalRootPayload(principal string, issuedAt, expiresAt int64, scopes []string) []byte {
	sortedScopes := SortScopes(scopes)
	serialized := fmt.Sprintf("root:%s:%d:%d:%s",
		strings.TrimSpace(principal),
		issuedAt,
		expiresAt,
		strings.Join(sortedScopes, ","),
	)
	return []byte(serialized)
}

// CanonicalHopPayload generates the deterministic UTF-8 bytes to sign/verify for a DelegationHop:
// hop:{hop_index}:{delegator_uri}:{delegatee_uri}:{prev_sig_hex}:{issued_at}:{expires_at}:{sorted_comma_separated_scopes}:{sorted_constraints}
func CanonicalHopPayload(
	hopIndex int,
	delegatorURI string,
	delegateeURI string,
	prevSigHex string,
	issuedAt int64,
	expiresAt int64,
	scopes []string,
	constraints map[string]string,
) []byte {
	sortedScopes := SortScopes(scopes)
	sortedConstraints := CanonicalConstraints(constraints)
	serialized := fmt.Sprintf("hop:%d:%s:%s:%s:%d:%d:%s:%s",
		hopIndex,
		strings.TrimSpace(delegatorURI),
		strings.TrimSpace(delegateeURI),
		strings.TrimSpace(prevSigHex),
		issuedAt,
		expiresAt,
		strings.Join(sortedScopes, ","),
		sortedConstraints,
	)
	return []byte(serialized)
}

// SortScopes returns a sorted copy of the scopes slice with whitespace trimmed.
func SortScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{}
	}
	res := make([]string, 0, len(scopes))
	for _, s := range scopes {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	sort.Strings(res)
	return res
}

// CanonicalConstraints returns sorted key=value pairs joined with commas: "k1=v1,k2=v2".
func CanonicalConstraints(constraints map[string]string) string {
	if len(constraints) == 0 {
		return ""
	}
	keys := make([]string, 0, len(constraints))
	for k := range constraints {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", strings.TrimSpace(k), strings.TrimSpace(constraints[k])))
	}
	return strings.Join(pairs, ",")
}

// SignRoot computes and sets the Ed25519 SignatureHex for a DelegationRoot.
func SignRoot(privKey ed25519.PrivateKey, root *DelegationRoot) error {
	if len(privKey) != ed25519.PrivateKeySize {
		return ErrInvalidPrivateKey
	}
	payload := CanonicalRootPayload(root.Principal, root.IssuedAt, root.ExpiresAt, root.InitialScopes)
	sig := ed25519.Sign(privKey, payload)
	root.SignatureHex = hex.EncodeToString(sig)
	return nil
}

// VerifyRootSignature verifies the Ed25519 SignatureHex of a DelegationRoot.
func VerifyRootSignature(pubKey ed25519.PublicKey, root *DelegationRoot) bool {
	if len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := hex.DecodeString(strings.TrimSpace(root.SignatureHex))
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	payload := CanonicalRootPayload(root.Principal, root.IssuedAt, root.ExpiresAt, root.InitialScopes)
	return ed25519.Verify(pubKey, payload, sigBytes)
}

// SignHop computes and sets the Ed25519 SignatureHex for a DelegationHop bound to prevSigHex.
func SignHop(privKey ed25519.PrivateKey, hop *DelegationHop, prevSigHex string) error {
	if len(privKey) != ed25519.PrivateKeySize {
		return ErrInvalidPrivateKey
	}
	payload := CanonicalHopPayload(
		hop.HopIndex,
		hop.DelegatorURI,
		hop.DelegateeURI,
		prevSigHex,
		hop.IssuedAt,
		hop.ExpiresAt,
		hop.AttenuatedScopes,
		hop.Constraints,
	)
	sig := ed25519.Sign(privKey, payload)
	hop.SignatureHex = hex.EncodeToString(sig)
	return nil
}

// VerifyHopSignature verifies the Ed25519 SignatureHex of a DelegationHop bound to prevSigHex.
func VerifyHopSignature(pubKey ed25519.PublicKey, hop *DelegationHop, prevSigHex string) bool {
	if len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := hex.DecodeString(strings.TrimSpace(hop.SignatureHex))
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	payload := CanonicalHopPayload(
		hop.HopIndex,
		hop.DelegatorURI,
		hop.DelegateeURI,
		prevSigHex,
		hop.IssuedAt,
		hop.ExpiresAt,
		hop.AttenuatedScopes,
		hop.Constraints,
	)
	return ed25519.Verify(pubKey, payload, sigBytes)
}
