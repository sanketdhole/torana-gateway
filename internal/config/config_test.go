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

func TestLoadBootstrapConfig_DebugAndLogLevel(t *testing.T) {
	// Case 1: default (no debug env)
	os.Unsetenv("DEBUG")
	os.Unsetenv("TORANA_DEBUG")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("TORANA_LOG_LEVEL")
	os.Unsetenv("LOG_FORMAT")
	os.Unsetenv("TORANA_LOG_FORMAT")

	cfg := LoadBootstrapConfig()
	if cfg.Debug {
		t.Errorf("expected default debug to be false, got true")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log_level to be info, got %s", cfg.LogLevel)
	}

	// Case 2: DEBUG=true
	os.Setenv("DEBUG", "true")
	cfg = LoadBootstrapConfig()
	if !cfg.Debug {
		t.Errorf("expected debug to be true with DEBUG=true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level to be debug with DEBUG=true, got %s", cfg.LogLevel)
	}
	os.Unsetenv("DEBUG")

	// Case 3: LOG_LEVEL=warn
	os.Setenv("LOG_LEVEL", "warn")
	cfg = LoadBootstrapConfig()
	if cfg.Debug {
		t.Errorf("expected debug to be false with LOG_LEVEL=warn")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log_level to be warn, got %s", cfg.LogLevel)
	}
	os.Unsetenv("LOG_LEVEL")

	// Case 4: LOG_LEVEL=debug
	os.Setenv("LOG_LEVEL", "debug")
	cfg = LoadBootstrapConfig()
	if !cfg.Debug {
		t.Errorf("expected debug to be true with LOG_LEVEL=debug")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level to be debug, got %s", cfg.LogLevel)
	}
	os.Unsetenv("LOG_LEVEL")

	// Case 5: TORANA_DEBUG=1 and TORANA_LOG_FORMAT=json
	os.Setenv("TORANA_DEBUG", "1")
	os.Setenv("TORANA_LOG_FORMAT", "json")
	cfg = LoadBootstrapConfig()
	if !cfg.Debug {
		t.Errorf("expected debug to be true with TORANA_DEBUG=1")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("expected log_format to be json, got %s", cfg.LogFormat)
	}
	os.Unsetenv("TORANA_DEBUG")
	os.Unsetenv("TORANA_LOG_FORMAT")
}

func TestParseFlags_DebugAndLogLevel(t *testing.T) {
	os.Unsetenv("DEBUG")
	os.Unsetenv("TORANA_DEBUG")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("TORANA_LOG_LEVEL")

	// Flag --debug
	cfg, _, err := ParseFlags([]string{"--debug"})
	if err != nil {
		t.Fatalf("unexpected ParseFlags error: %v", err)
	}
	if !cfg.Debug {
		t.Errorf("expected cfg.Debug to be true with --debug flag")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected cfg.LogLevel to be debug with --debug flag, got %s", cfg.LogLevel)
	}

	// Flag --log-level=warn
	cfg, _, err = ParseFlags([]string{"--log-level", "warn"})
	if err != nil {
		t.Fatalf("unexpected ParseFlags error: %v", err)
	}
	if cfg.Debug {
		t.Errorf("expected cfg.Debug to be false with --log-level warn")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected cfg.LogLevel to be warn, got %s", cfg.LogLevel)
	}

	// Flag --log-level=debug
	cfg, _, err = ParseFlags([]string{"--log-level", "debug", "--log-format", "text"})
	if err != nil {
		t.Fatalf("unexpected ParseFlags error: %v", err)
	}
	if !cfg.Debug {
		t.Errorf("expected cfg.Debug to be true with --log-level debug")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected cfg.LogLevel to be debug, got %s", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("expected cfg.LogFormat to be text, got %s", cfg.LogFormat)
	}
}

func TestResolveAndPersistNodeID_PersistsAcrossRestarts(t *testing.T) {
	tempDir := t.TempDir()
	nodeFile := filepath.Join(tempDir, "node_id")

	// First start: empty explicit ID, should generate and persist
	id1 := ResolveAndPersistNodeID("", nodeFile, "prod")
	if id1 == "" {
		t.Fatalf("expected non-empty generated node ID")
	}

	// Verify file was written
	data, err := os.ReadFile(nodeFile)
	if err != nil {
		t.Fatalf("expected node ID file to exist: %v", err)
	}
	if string(data) != id1+"\n" {
		t.Errorf("expected file content %q, got %q", id1+"\n", string(data))
	}

	// Second start (simulating container restart): should reuse the exact same ID from file
	id2 := ResolveAndPersistNodeID("", nodeFile, "prod")
	if id1 != id2 {
		t.Errorf("expected node ID to match across restarts: %q != %q", id1, id2)
	}
}

func TestResolveAndPersistNodeID_ExplicitOverridesAndPersists(t *testing.T) {
	tempDir := t.TempDir()
	nodeFile := filepath.Join(tempDir, "node_id")

	explicitID := "gateway-custom-instance-01"
	id := ResolveAndPersistNodeID(explicitID, nodeFile, "prod")
	if id != explicitID {
		t.Errorf("expected %q, got %q", explicitID, id)
	}

	// Restart with empty explicit ID should now return the previously saved explicit ID
	restartedID := ResolveAndPersistNodeID("", nodeFile, "prod")
	if restartedID != explicitID {
		t.Errorf("expected %q across restart, got %q", explicitID, restartedID)
	}
}

func TestParseFlags_NodeIDAndInstanceName(t *testing.T) {
	tempDir := t.TempDir()
	nodeFile := filepath.Join(tempDir, "node_id")

	// Test --node-id flag
	cfg, _, err := ParseFlags([]string{"--node-id", "my-node-42", "--node-id-file", nodeFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.NodeID != "my-node-42" {
		t.Errorf("expected node ID my-node-42, got %s", cfg.NodeID)
	}

	// Test --instance-name flag alias
	cfg2, _, err := ParseFlags([]string{"--instance-name", "my-instance-99", "--node-id-file", nodeFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.NodeID != "my-instance-99" {
		t.Errorf("expected node ID my-instance-99, got %s", cfg2.NodeID)
	}
}

func TestNodeIDPrecedence_FlagOverEnvOverFile(t *testing.T) {
	tempDir := t.TempDir()
	nodeFile := filepath.Join(tempDir, "node_id")

	// 1. File only
	_ = os.WriteFile(nodeFile, []byte("id-from-file\n"), 0644)
	os.Unsetenv("TORANA_NODE_ID")
	os.Unsetenv("NODE_ID")
	os.Setenv("TORANA_NODE_ID_FILE", nodeFile)
	defer func() {
		os.Unsetenv("TORANA_NODE_ID")
		os.Unsetenv("TORANA_NODE_ID_FILE")
	}()

	cfg1 := LoadBootstrapConfig()
	if cfg1.NodeID != "id-from-file" {
		t.Errorf("expected id-from-file, got %s", cfg1.NodeID)
	}

	// 2. Env over file
	os.Setenv("TORANA_NODE_ID", "id-from-env")
	cfg2 := LoadBootstrapConfig()
	if cfg2.NodeID != "id-from-env" {
		t.Errorf("expected id-from-env, got %s", cfg2.NodeID)
	}

	// 3. Flag over env and file
	cfg3, _, err := ParseFlags([]string{"-node-id", "id-from-flag", "-node-id-file", nodeFile})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if cfg3.NodeID != "id-from-flag" {
		t.Errorf("expected id-from-flag, got %s", cfg3.NodeID)
	}
}


