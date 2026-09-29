package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phaselume/torana/pkg/bootstrap"
)

func TestLoadBootstrapConfig_FromEnvToken(t *testing.T) {
	// Clear relevant env vars
	envKeys := []string{
		"TORANA_TOKEN", "BOOTSTRAP_TOKEN", "TORANA_ORG_ID", "PLATFORM_URL",
		"TORANA_PLATFORM_URL", "ENROLL_TOKEN", "TORANA_ENROLL_TOKEN",
		"ENROLL_TOKEN_FILE", "GATEWAY_NAMESPACE", "TORANA_NAMESPACE",
	}
	for _, k := range envKeys {
		os.Unsetenv(k)
	}

	payload := &bootstrap.BootstrapPayload{
		PlatformURL: "grpc.torana.cloud:443",
		OrgID:       "org_acme_prod",
		Namespace:   "production",
		EnrollToken: "torana-enroll-token-12345",
		CreatedAt:   1727600000,
	}

	tokenStr, err := bootstrap.EncodeBootstrapToken(payload)
	if err != nil {
		t.Fatalf("failed to encode token: %v", err)
	}

	os.Setenv("TORANA_TOKEN", tokenStr)
	defer os.Unsetenv("TORANA_TOKEN")

	cfg := LoadBootstrapConfig()

	if cfg.PlatformURL != "grpc.torana.cloud:443" {
		t.Errorf("expected platform_url grpc.torana.cloud:443, got %s", cfg.PlatformURL)
	}
	if cfg.OrgID != "org_acme_prod" {
		t.Errorf("expected org_id org_acme_prod, got %s", cfg.OrgID)
	}
	if cfg.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", cfg.Namespace)
	}
	if cfg.EnrollToken != "torana-enroll-token-12345" {
		t.Errorf("expected enroll_token torana-enroll-token-12345, got %s", cfg.EnrollToken)
	}
	if cfg.Token != tokenStr {
		t.Errorf("expected token %s, got %s", tokenStr, cfg.Token)
	}
}

func TestLoadBootstrapConfig_EnvOverridesToken(t *testing.T) {
	envKeys := []string{
		"TORANA_TOKEN", "BOOTSTRAP_TOKEN", "TORANA_ORG_ID", "PLATFORM_URL",
		"TORANA_PLATFORM_URL", "ENROLL_TOKEN", "TORANA_ENROLL_TOKEN",
		"ENROLL_TOKEN_FILE", "GATEWAY_NAMESPACE", "TORANA_NAMESPACE",
	}
	for _, k := range envKeys {
		os.Unsetenv(k)
	}

	payload := &bootstrap.BootstrapPayload{
		PlatformURL: "grpc.torana.cloud:443",
		OrgID:       "org_token_id",
		Namespace:   "token_namespace",
		EnrollToken: "token_enroll_secret",
		CreatedAt:   1727600000,
	}

	tokenStr, err := bootstrap.EncodeBootstrapToken(payload)
	if err != nil {
		t.Fatalf("failed to encode token: %v", err)
	}

	os.Setenv("TORANA_TOKEN", tokenStr)
	os.Setenv("TORANA_PLATFORM_URL", "custom.platform.override:8443")
	os.Setenv("TORANA_NAMESPACE", "custom-namespace-override")
	defer func() {
		os.Unsetenv("TORANA_TOKEN")
		os.Unsetenv("TORANA_PLATFORM_URL")
		os.Unsetenv("TORANA_NAMESPACE")
	}()

	cfg := LoadBootstrapConfig()

	// PlatformURL and Namespace should take values from explicit env vars
	if cfg.PlatformURL != "custom.platform.override:8443" {
		t.Errorf("expected platform_url custom.platform.override:8443, got %s", cfg.PlatformURL)
	}
	if cfg.Namespace != "custom-namespace-override" {
		t.Errorf("expected namespace custom-namespace-override, got %s", cfg.Namespace)
	}
	// OrgID and EnrollToken should still fall back to token payload
	if cfg.OrgID != "org_token_id" {
		t.Errorf("expected org_id org_token_id, got %s", cfg.OrgID)
	}
	if cfg.EnrollToken != "token_enroll_secret" {
		t.Errorf("expected enroll_token token_enroll_secret, got %s", cfg.EnrollToken)
	}
}

func TestParseFlags_TokenFlag(t *testing.T) {
	envKeys := []string{
		"TORANA_TOKEN", "BOOTSTRAP_TOKEN", "TORANA_ORG_ID", "PLATFORM_URL",
		"TORANA_PLATFORM_URL", "ENROLL_TOKEN", "TORANA_ENROLL_TOKEN",
		"ENROLL_TOKEN_FILE", "GATEWAY_NAMESPACE", "TORANA_NAMESPACE",
	}
	for _, k := range envKeys {
		os.Unsetenv(k)
	}

	payload := &bootstrap.BootstrapPayload{
		PlatformURL: "platform.internal:9090",
		OrgID:       "org_flag_test",
		Namespace:   "staging",
		EnrollToken: "staging-enroll-abc",
		CreatedAt:   1727600000,
	}

	tokenStr, err := bootstrap.EncodeBootstrapToken(payload)
	if err != nil {
		t.Fatalf("failed to encode token: %v", err)
	}

	args := []string{"--token", tokenStr}
	cfg, _, err := ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected ParseFlags error: %v", err)
	}

	if cfg.PlatformURL != "platform.internal:9090" {
		t.Errorf("expected platform_url platform.internal:9090, got %s", cfg.PlatformURL)
	}
	if cfg.OrgID != "org_flag_test" {
		t.Errorf("expected org_id org_flag_test, got %s", cfg.OrgID)
	}
	if cfg.Namespace != "staging" {
		t.Errorf("expected namespace staging, got %s", cfg.Namespace)
	}
	if cfg.EnrollToken != "staging-enroll-abc" {
		t.Errorf("expected enroll_token staging-enroll-abc, got %s", cfg.EnrollToken)
	}
}

func TestParseFlags_ExplicitFlagsPrecedence(t *testing.T) {
	envKeys := []string{
		"TORANA_TOKEN", "BOOTSTRAP_TOKEN", "TORANA_ORG_ID", "PLATFORM_URL",
		"TORANA_PLATFORM_URL", "ENROLL_TOKEN", "TORANA_ENROLL_TOKEN",
		"ENROLL_TOKEN_FILE", "GATEWAY_NAMESPACE", "TORANA_NAMESPACE",
	}
	for _, k := range envKeys {
		os.Unsetenv(k)
	}

	payload := &bootstrap.BootstrapPayload{
		PlatformURL: "grpc.torana.cloud:443",
		OrgID:       "org_token_val",
		Namespace:   "token-namespace",
		EnrollToken: "token-enroll-token",
		CreatedAt:   1727600000,
	}

	tokenStr, err := bootstrap.EncodeBootstrapToken(payload)
	if err != nil {
		t.Fatalf("failed to encode token: %v", err)
	}

	// Explicit CLI flags override the fields extracted from the bootstrap token
	args := []string{
		"--token", tokenStr,
		"--platform-url", "override.platform:8443",
		"--namespace", "override-namespace",
		"--enroll-token", "override-enroll-token",
		"--org-id", "override-org",
	}

	cfg, _, err := ParseFlags(args)
	if err != nil {
		t.Fatalf("unexpected ParseFlags error: %v", err)
	}

	if cfg.PlatformURL != "override.platform:8443" {
		t.Errorf("expected overridden platform_url override.platform:8443, got %s", cfg.PlatformURL)
	}
	if cfg.Namespace != "override-namespace" {
		t.Errorf("expected overridden namespace override-namespace, got %s", cfg.Namespace)
	}
	if cfg.EnrollToken != "override-enroll-token" {
		t.Errorf("expected overridden enroll_token override-enroll-token, got %s", cfg.EnrollToken)
	}
	if cfg.OrgID != "override-org" {
		t.Errorf("expected overridden org_id override-org, got %s", cfg.OrgID)
	}
}

func TestGetEnrollToken(t *testing.T) {
	// Case 1: direct EnrollToken string
	cfg1 := &BootstrapConfig{
		EnrollToken: "direct-token-string",
	}
	tok := cfg1.GetEnrollToken()
	if tok != "direct-token-string" {
		t.Errorf("expected direct-token-string, got %s", tok)
	}

	// Case 2: from EnrollTokenFile
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.secret")
	if err := os.WriteFile(tokenFile, []byte("file-token-secret\n"), 0600); err != nil {
		t.Fatalf("failed to write test token file: %v", err)
	}

	cfg2 := &BootstrapConfig{
		EnrollTokenFile: tokenFile,
	}
	tok2 := cfg2.GetEnrollToken()
	if tok2 != "file-token-secret" {
		t.Errorf("expected file-token-secret, got %s", tok2)
	}

	// Case 3: neither provided
	cfg3 := &BootstrapConfig{}
	tok3 := cfg3.GetEnrollToken()
	if tok3 != "" {
		t.Errorf("expected empty token when neither is set, got %s", tok3)
	}
}
