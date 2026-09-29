package bootstrap

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrEmptyToken is returned when the bootstrap token string is empty.
	ErrEmptyToken = errors.New("bootstrap token cannot be empty")
	// ErrInvalidBase64 is returned when the token string cannot be decoded by any supported Base64 encoding.
	ErrInvalidBase64 = errors.New("failed to decode bootstrap token base64")
	// ErrInvalidJSON is returned when the decoded bytes cannot be parsed as JSON.
	ErrInvalidJSON = errors.New("failed to unmarshal bootstrap token json")
	// ErrMissingPlatformURL is returned when platform_url is empty.
	ErrMissingPlatformURL = errors.New("bootstrap token missing required field 'platform_url'")
	// ErrMissingOrgID is returned when org_id is empty.
	ErrMissingOrgID = errors.New("bootstrap token missing required field 'org_id'")
	// ErrMissingNamespace is returned when namespace is empty.
	ErrMissingNamespace = errors.New("bootstrap token missing required field 'namespace'")
	// ErrMissingEnrollToken is returned when enroll_token is empty.
	ErrMissingEnrollToken = errors.New("bootstrap token missing required field 'enroll_token'")
)

// BootstrapPayload contains the connection parameters packaged inside TORANA_TOKEN.
type BootstrapPayload struct {
	PlatformURL string `json:"platform_url"`
	OrgID       string `json:"org_id"`
	Namespace   string `json:"namespace"`
	EnrollToken string `json:"enroll_token"`
	CreatedAt   int64  `json:"created_at"`
}

// DecodeBootstrapToken decodes a URL-safe or standard Base64 bootstrap token string into BootstrapPayload.
// It supports optional "tt_" prefix, trims whitespace, and validates all required fields.
func DecodeBootstrapToken(tokenStr string) (*BootstrapPayload, error) {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return nil, ErrEmptyToken
	}

	// Strip optional "tt_" prefix if present
	tokenStr = strings.TrimPrefix(tokenStr, "tt_")

	// Attempt Base64 decodings across all common URL and Standard encodings
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}

	var raw []byte
	var decoded bool
	for _, enc := range encodings {
		if data, err := enc.DecodeString(tokenStr); err == nil {
			raw = data
			decoded = true
			break
		}
	}

	if !decoded {
		return nil, fmt.Errorf("%w: invalid encoding format", ErrInvalidBase64)
	}

	var payload BootstrapPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	// Validate required fields
	if strings.TrimSpace(payload.PlatformURL) == "" {
		return nil, ErrMissingPlatformURL
	}
	if strings.TrimSpace(payload.OrgID) == "" {
		return nil, ErrMissingOrgID
	}
	if strings.TrimSpace(payload.Namespace) == "" {
		return nil, ErrMissingNamespace
	}
	if strings.TrimSpace(payload.EnrollToken) == "" {
		return nil, ErrMissingEnrollToken
	}

	return &payload, nil
}

// EncodeBootstrapToken serializes a BootstrapPayload into a URL-safe Base64 token string.
func EncodeBootstrapToken(payload *BootstrapPayload) (string, error) {
	if payload == nil {
		return "", errors.New("payload cannot be nil")
	}
	if payload.CreatedAt == 0 {
		payload.CreatedAt = time.Now().Unix()
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal bootstrap payload: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}
