package bootstrap_test

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/phaselume/torana/pkg/bootstrap"
)

func TestDecodeBootstrapToken(t *testing.T) {
	validPayload := &bootstrap.BootstrapPayload{
		PlatformURL: "grpc.torana.cloud:443",
		OrgID:       "org_acme_prod",
		Namespace:   "production",
		EnrollToken: "torana-enroll-production-7f89d3",
		CreatedAt:   time.Now().Unix(),
	}

	t.Run("URL-Safe Base64 without padding", func(t *testing.T) {
		tokenStr, err := bootstrap.EncodeBootstrapToken(validPayload)
		if err != nil {
			t.Fatalf("EncodeBootstrapToken failed: %v", err)
		}

		decoded, err := bootstrap.DecodeBootstrapToken(tokenStr)
		if err != nil {
			t.Fatalf("DecodeBootstrapToken failed: %v", err)
		}

		if decoded.PlatformURL != validPayload.PlatformURL {
			t.Errorf("expected platform_url %s, got %s", validPayload.PlatformURL, decoded.PlatformURL)
		}
		if decoded.OrgID != validPayload.OrgID {
			t.Errorf("expected org_id %s, got %s", validPayload.OrgID, decoded.OrgID)
		}
		if decoded.Namespace != validPayload.Namespace {
			t.Errorf("expected namespace %s, got %s", validPayload.Namespace, decoded.Namespace)
		}
		if decoded.EnrollToken != validPayload.EnrollToken {
			t.Errorf("expected enroll_token %s, got %s", validPayload.EnrollToken, decoded.EnrollToken)
		}
	})

	t.Run("Standard Base64 with padding", func(t *testing.T) {
		jsonBytes := []byte(`{"platform_url":"grpc.torana.cloud:443","org_id":"org_acme_prod","namespace":"production","enroll_token":"tok_123"}`)
		stdToken := base64.StdEncoding.EncodeToString(jsonBytes)

		decoded, err := bootstrap.DecodeBootstrapToken(stdToken)
		if err != nil {
			t.Fatalf("DecodeBootstrapToken failed for standard base64: %v", err)
		}
		if decoded.PlatformURL != "grpc.torana.cloud:443" || decoded.OrgID != "org_acme_prod" {
			t.Errorf("unexpected decoded payload: %+v", decoded)
		}
	})

	t.Run("Optional tt_ prefix and whitespace", func(t *testing.T) {
		tokenStr, err := bootstrap.EncodeBootstrapToken(validPayload)
		if err != nil {
			t.Fatalf("EncodeBootstrapToken failed: %v", err)
		}

		prefixedToken := "  tt_" + tokenStr + " \n"
		decoded, err := bootstrap.DecodeBootstrapToken(prefixedToken)
		if err != nil {
			t.Fatalf("DecodeBootstrapToken failed with tt_ prefix: %v", err)
		}
		if decoded.Namespace != "production" {
			t.Errorf("expected namespace production, got %s", decoded.Namespace)
		}
	})

	t.Run("Empty token error", func(t *testing.T) {
		_, err := bootstrap.DecodeBootstrapToken("   ")
		if !errors.Is(err, bootstrap.ErrEmptyToken) {
			t.Errorf("expected ErrEmptyToken, got: %v", err)
		}
	})

	t.Run("Corrupt base64 error", func(t *testing.T) {
		_, err := bootstrap.DecodeBootstrapToken("!!invalid-base64!!")
		if !errors.Is(err, bootstrap.ErrInvalidBase64) {
			t.Errorf("expected ErrInvalidBase64, got: %v", err)
		}
	})

	t.Run("Non-JSON payload error", func(t *testing.T) {
		raw := base64.RawURLEncoding.EncodeToString([]byte("plain non json string"))
		_, err := bootstrap.DecodeBootstrapToken(raw)
		if !errors.Is(err, bootstrap.ErrInvalidJSON) {
			t.Errorf("expected ErrInvalidJSON, got: %v", err)
		}
	})

	t.Run("Missing required field errors", func(t *testing.T) {
		tests := []struct {
			name        string
			payloadJSON string
			expectedErr error
		}{
			{
				name:        "missing platform_url",
				payloadJSON: `{"org_id":"o1","namespace":"ns1","enroll_token":"t1"}`,
				expectedErr: bootstrap.ErrMissingPlatformURL,
			},
			{
				name:        "missing org_id",
				payloadJSON: `{"platform_url":"localhost:9090","namespace":"ns1","enroll_token":"t1"}`,
				expectedErr: bootstrap.ErrMissingOrgID,
			},
			{
				name:        "missing namespace",
				payloadJSON: `{"platform_url":"localhost:9090","org_id":"o1","enroll_token":"t1"}`,
				expectedErr: bootstrap.ErrMissingNamespace,
			},
			{
				name:        "missing enroll_token",
				payloadJSON: `{"platform_url":"localhost:9090","org_id":"o1","namespace":"ns1"}`,
				expectedErr: bootstrap.ErrMissingEnrollToken,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				tok := base64.RawURLEncoding.EncodeToString([]byte(tc.payloadJSON))
				_, err := bootstrap.DecodeBootstrapToken(tok)
				if !errors.Is(err, tc.expectedErr) {
					t.Errorf("expected %v, got %v", tc.expectedErr, err)
				}
			})
		}
	})
}
