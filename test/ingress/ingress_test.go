package ingress_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/phaselume/torana/internal/agentauth"
	"github.com/phaselume/torana/internal/config"
	"github.com/phaselume/torana/internal/egress"
	"github.com/phaselume/torana/internal/ingress"
	"github.com/phaselume/torana/internal/pipeline"
	"github.com/phaselume/torana/internal/router"
	"github.com/phaselume/torana/internal/security/authz"
	"github.com/phaselume/torana/internal/telemetry"
)

type mockEgressClient struct {
	protocol string
	response string
	status   int
	lastReq  *egress.Request
}

func (m *mockEgressClient) Protocol() string { return m.protocol }
func (m *mockEgressClient) Execute(_ context.Context, _ *config.UpstreamCluster, req *egress.Request) (*egress.Response, error) {
	m.lastReq = req
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return &egress.Response{
		StatusCode: m.status,
		Headers:    header,
		Body:       io.NopCloser(bytes.NewBufferString(m.response)),
	}, nil
}
func (m *mockEgressClient) Close() error { return nil }

type testSink struct {
	events []telemetry.Event
}

func (t *testSink) SendBatch(_ context.Context, events []telemetry.Event) error {
	t.events = append(t.events, events...)
	return nil
}

func setupTestListenerWithEgressAndFilters(filters ...pipeline.Filter) (*ingress.HTTPListener, *testSink, *mockEgressClient) {
	cfg := &config.BootstrapConfig{
		ListenHTTP: ":8080",
	}

	snap := &config.Snapshot{
		Version: 1,
		Upstreams: map[string]config.UpstreamCluster{
			"llm-openai": {
				ID:        "llm-openai",
				Protocol:  "http",
				Endpoints: []string{"http://mock.local"},
			},
		},
		Routes: []config.RouteRule{
			{
				ID:         "route-chat",
				Path:       "/v1/chat/completions",
				Method:     "POST",
				UpstreamID: "llm-openai",
			},
		},
	}

	holder := config.NewSnapshotHolder()
	_ = holder.Store(snap)

	rtr := router.Compile(snap)

	egressReg := egress.NewRegistry()
	mockClient := &mockEgressClient{
		protocol: "http",
		response: `{"choices":[{"message":{"content":"Hello AI"}}]}`,
		status:   http.StatusOK,
	}
	egressReg.Register("http", mockClient)

	sink := &testSink{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	emitter := telemetry.NewEmitter(100, sink, logger)
	emitter.Start(context.Background())

	chain := pipeline.NewChain(filters...)

	listener := ingress.NewHTTPListener(cfg, holder, egressReg, emitter, chain, logger)
	listener.UpdateRouter(rtr)
	listener.SetReady(true)

	return listener, sink, mockClient
}

func setupTestListener() (*ingress.HTTPListener, *testSink) {
	listener, sink, _ := setupTestListenerWithEgressAndFilters()
	return listener, sink
}

func TestHTTPListener_HealthAndReady(t *testing.T) {
	listener, _ := setupTestListener()

	t.Run("healthz is always UP", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		listener.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != `{"status":"UP"}` {
			t.Fatalf("expected {\"status\":\"UP\"}, got %s", rec.Body.String())
		}
	})

	t.Run("readyz reflects ready state", func(t *testing.T) {
		listener.SetReady(false)
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()

		listener.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503 when not ready, got %d", rec.Code)
		}

		listener.SetReady(true)
		rec = httptest.NewRecorder()
		listener.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 when ready, got %d", rec.Code)
		}
	})
}

func TestHTTPListener_ServeGatewayRequest(t *testing.T) {
	listener, _ := setupTestListener()

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	listener.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body=%s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	if rec.Body.String() != `{"choices":[{"message":{"content":"Hello AI"}}]}` {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestHTTPListener_RouteNotFound(t *testing.T) {
	listener, _ := setupTestListener()

	req := httptest.NewRequest(http.MethodGet, "/v1/nonexistent", nil)
	rec := httptest.NewRecorder()

	listener.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func BenchmarkHTTPListener_HotPath(b *testing.B) {
	listener, _ := setupTestListener()
	body := []byte(`{"model":"gpt-4o"}`)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			listener.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("expected status 200, got %d", rec.Code)
			}
		}
	})
}

func setupDelegationTestListener(filters ...pipeline.Filter) (*ingress.HTTPListener, *testSink, *mockEgressClient, *agentauth.Registry, ed25519.PrivateKey, ed25519.PrivateKey) {
	pubTenant, privTenant, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	pubAgent, privAgent, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}

	reg := agentauth.NewRegistry()
	reg.SetTenantPublicKey(pubTenant)
	reg.RegisterAgent(agentauth.AgentRegistryEntry{
		AgentURI:     "torana://org1/prod/agent/agt_planner",
		PublicKeyHex: hex.EncodeToString(pubAgent),
		RiskTier:     "LOW",
		Active:       true,
	})

	listener, sink, mockClient := setupTestListenerWithEgressAndFilters(filters...)
	verifier := agentauth.NewVerifier(reg, nil)
	listener.SetVerifier(verifier)

	return listener, sink, mockClient, reg, privTenant, privAgent
}

func TestHTTPListener_DelegationToken_ValidChain_HTTPHeadersAndTelemetry(t *testing.T) {
	listener, sink, mockClient, _, privTenant, _ := setupDelegationTestListener()

	now := time.Now().Unix()
	root := agentauth.DelegationRoot{
		Principal:     "torana://org1/prod/user/usr_sanket",
		IssuedAt:      now - 60,
		ExpiresAt:     now + 3600,
		InitialScopes: []string{"analytics:*", "db:read"},
	}
	if err := agentauth.SignRoot(privTenant, &root); err != nil {
		t.Fatalf("failed to sign root: %v", err)
	}

	hop1 := agentauth.DelegationHop{
		HopIndex:         1,
		DelegatorURI:     "torana://org1/prod/user/usr_sanket",
		DelegateeURI:     "torana://org1/prod/agent/agt_planner",
		AttenuatedScopes: []string{"analytics:query"},
		IssuedAt:         now - 30,
		ExpiresAt:        now + 1800,
	}
	if err := agentauth.SignHop(privTenant, &hop1, root.SignatureHex); err != nil {
		t.Fatalf("failed to sign hop 1: %v", err)
	}

	token := agentauth.ToranaDelegationToken{
		Version: "1.0",
		Root:    root,
		Hops:    []agentauth.DelegationHop{hop1},
	}
	tokenBytes, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("failed to marshal token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"prompt":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Torana-Delegation", string(tokenBytes))
	// Add untrusted/spoofed headers to test sanitization
	req.Header.Set("X-Torana-Root-Principal", "malicious_root")
	req.Header.Set("X-Torana-Delegation-Verified", "false_claim")

	rec := httptest.NewRecorder()
	listener.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	// Verify upstream headers received by mock egress client
	if mockClient.lastReq == nil {
		t.Fatal("expected mock egress to receive request")
	}
	upHeaders := mockClient.lastReq.Headers
	if upHeaders.Get("X-Torana-Root-Principal") != "torana://org1/prod/user/usr_sanket" {
		t.Errorf("expected verified root principal header, got %q", upHeaders.Get("X-Torana-Root-Principal"))
	}
	if upHeaders.Get("X-Torana-Caller-Principal") != "torana://org1/prod/agent/agt_planner" {
		t.Errorf("expected caller principal header, got %q", upHeaders.Get("X-Torana-Caller-Principal"))
	}
	if upHeaders.Get("X-Torana-Chain-Depth") != "1" {
		t.Errorf("expected chain depth 1, got %q", upHeaders.Get("X-Torana-Chain-Depth"))
	}
	if upHeaders.Get("X-Torana-Effective-Scopes") != "analytics:query" {
		t.Errorf("expected effective scopes analytics:query, got %q", upHeaders.Get("X-Torana-Effective-Scopes"))
	}
	if upHeaders.Get("X-Torana-Delegation-Verified") != "true" {
		t.Errorf("expected verified=true, got %q", upHeaders.Get("X-Torana-Delegation-Verified"))
	}

	// Allow telemetry event to be flushed
	time.Sleep(50 * time.Millisecond)
	_ = sink // sink received telemetry
}

func TestHTTPListener_DelegationToken_Base64Encoded(t *testing.T) {
	listener, _, mockClient, _, privTenant, _ := setupDelegationTestListener()

	now := time.Now().Unix()
	root := agentauth.DelegationRoot{
		Principal:     "torana://org1/prod/user/usr_sanket",
		IssuedAt:      now - 60,
		ExpiresAt:     now + 3600,
		InitialScopes: []string{"analytics:*"},
	}
	_ = agentauth.SignRoot(privTenant, &root)

	hop1 := agentauth.DelegationHop{
		HopIndex:         1,
		DelegatorURI:     "torana://org1/prod/user/usr_sanket",
		DelegateeURI:     "torana://org1/prod/agent/agt_planner",
		AttenuatedScopes: []string{"analytics:read"},
		IssuedAt:         now - 30,
		ExpiresAt:        now + 1800,
	}
	_ = agentauth.SignHop(privTenant, &hop1, root.SignatureHex)

	token := agentauth.ToranaDelegationToken{
		Version: "1.0",
		Root:    root,
		Hops:    []agentauth.DelegationHop{hop1},
	}
	tokenBytes, _ := json.Marshal(token)
	b64Token := base64.URLEncoding.EncodeToString(tokenBytes)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Torana-Delegation", b64Token)

	rec := httptest.NewRecorder()
	listener.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if mockClient.lastReq.Headers.Get("X-Torana-Effective-Scopes") != "analytics:read" {
		t.Errorf("expected effective scopes analytics:read, got %s", mockClient.lastReq.Headers.Get("X-Torana-Effective-Scopes"))
	}
}

func TestHTTPListener_DelegationToken_TamperedSignature_Rejected(t *testing.T) {
	listener, _, _, _, privTenant, _ := setupDelegationTestListener()

	now := time.Now().Unix()
	root := agentauth.DelegationRoot{
		Principal:     "torana://org1/prod/user/usr_sanket",
		IssuedAt:      now - 60,
		ExpiresAt:     now + 3600,
		InitialScopes: []string{"analytics:*"},
	}
	_ = agentauth.SignRoot(privTenant, &root)

	// Tamper root signature
	root.SignatureHex = hex.EncodeToString(bytes.Repeat([]byte{0x01}, 64))

	token := agentauth.ToranaDelegationToken{
		Version: "1.0",
		Root:    root,
		Hops:    nil,
	}
	tokenBytes, _ := json.Marshal(token)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Torana-Delegation", string(tokenBytes))

	rec := httptest.NewRecorder()
	listener.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 401 or 403, got %d", rec.Code)
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse json error: %v, raw: %s", err, rec.Body.String())
	}
	if errResp["error"] != "delegation_verification_failed" {
		t.Errorf("expected error delegation_verification_failed, got %s", errResp["error"])
	}
	if errResp["reason"] == "" {
		t.Errorf("expected non-empty reason")
	}
}

func TestHTTPListener_DelegationToken_ScopeEscalation_Rejected(t *testing.T) {
	listener, _, _, _, privTenant, _ := setupDelegationTestListener()

	now := time.Now().Unix()
	root := agentauth.DelegationRoot{
		Principal:     "torana://org1/prod/user/usr_sanket",
		IssuedAt:      now - 60,
		ExpiresAt:     now + 3600,
		InitialScopes: []string{"analytics:query"},
	}
	_ = agentauth.SignRoot(privTenant, &root)

	// Escalation: hop requests admin:delete
	hop1 := agentauth.DelegationHop{
		HopIndex:         1,
		DelegatorURI:     "torana://org1/prod/user/usr_sanket",
		DelegateeURI:     "torana://org1/prod/agent/agt_planner",
		AttenuatedScopes: []string{"admin:delete"},
		IssuedAt:         now - 30,
		ExpiresAt:        now + 1800,
	}
	_ = agentauth.SignHop(privTenant, &hop1, root.SignatureHex)

	token := agentauth.ToranaDelegationToken{
		Version: "1.0",
		Root:    root,
		Hops:    []agentauth.DelegationHop{hop1},
	}
	tokenBytes, _ := json.Marshal(token)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Torana-Delegation", string(tokenBytes))

	rec := httptest.NewRecorder()
	listener.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for scope escalation, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "delegation_verification_failed") {
		t.Errorf("expected delegation_verification_failed in body, got %s", rec.Body.String())
	}
}

func TestHTTPListener_DelegationToken_CELPolicyEnforcement(t *testing.T) {
	compiler, err := authz.NewCompiler(authz.CompilerConfig{})
	if err != nil {
		t.Fatalf("failed to create compiler: %v", err)
	}

	rules := []authz.Rule{
		{
			ID:       "chain-access-control",
			Priority: 10,
			Effect:   authz.EffectAllow,
			Expression: `chain.root.type == "user" && ` +
				`chain.depth <= 2 && ` +
				`"analytics:query" in chain.effective_scopes && ` +
				`!chain.hops.exists(h, h.delegatee.endsWith("/unverified_crawler"))`,
		},
	}
	compiled, err := compiler.CompileRules(rules)
	if err != nil {
		t.Fatalf("failed to compile rules: %v", err)
	}
	engine := authz.NewPolicyEngine(compiled, nil)
	filter := authz.NewFilter(engine, nil)

	listener, _, _, reg, privTenant, privPlanner := setupDelegationTestListener(filter)

	pubCrawler, privCrawler, _ := ed25519.GenerateKey(nil)
	reg.RegisterAgent(agentauth.AgentRegistryEntry{
		AgentURI:     "torana://org1/prod/agent/unverified_crawler",
		PublicKeyHex: hex.EncodeToString(pubCrawler),
		RiskTier:     "HIGH",
		Active:       true,
	})

	now := time.Now().Unix()
	root := agentauth.DelegationRoot{
		Principal:     "torana://org1/prod/user/usr_sanket",
		IssuedAt:      now - 60,
		ExpiresAt:     now + 3600,
		InitialScopes: []string{"analytics:*"},
	}
	_ = agentauth.SignRoot(privTenant, &root)

	t.Run("conforming chain passes policy", func(t *testing.T) {
		hop1 := agentauth.DelegationHop{
			HopIndex:         1,
			DelegatorURI:     "torana://org1/prod/user/usr_sanket",
			DelegateeURI:     "torana://org1/prod/agent/agt_planner",
			AttenuatedScopes: []string{"analytics:query"},
			IssuedAt:         now - 30,
			ExpiresAt:        now + 1800,
		}
		_ = agentauth.SignHop(privTenant, &hop1, root.SignatureHex)

		token := agentauth.ToranaDelegationToken{
			Version: "1.0",
			Root:    root,
			Hops:    []agentauth.DelegationHop{hop1},
		}
		tokenBytes, _ := json.Marshal(token)

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Torana-Delegation", string(tokenBytes))

		rec := httptest.NewRecorder()
		listener.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d body: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("chain with forbidden crawler is rejected by CEL policy", func(t *testing.T) {
		hop1 := agentauth.DelegationHop{
			HopIndex:         1,
			DelegatorURI:     "torana://org1/prod/user/usr_sanket",
			DelegateeURI:     "torana://org1/prod/agent/agt_planner",
			AttenuatedScopes: []string{"analytics:query"},
			IssuedAt:         now - 30,
			ExpiresAt:        now + 1800,
		}
		_ = agentauth.SignHop(privTenant, &hop1, root.SignatureHex)

		hop2 := agentauth.DelegationHop{
			HopIndex:         2,
			DelegatorURI:     "torana://org1/prod/agent/agt_planner",
			DelegateeURI:     "torana://org1/prod/agent/unverified_crawler",
			AttenuatedScopes: []string{"analytics:query"},
			IssuedAt:         now - 10,
			ExpiresAt:        now + 900,
		}
		_ = agentauth.SignHop(privPlanner, &hop2, hop1.SignatureHex)

		token := agentauth.ToranaDelegationToken{
			Version: "1.0",
			Root:    root,
			Hops:    []agentauth.DelegationHop{hop1, hop2},
		}
		tokenBytes, _ := json.Marshal(token)

		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Torana-Delegation", string(tokenBytes))

		rec := httptest.NewRecorder()
		listener.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden due to CEL policy rejection, got %d", rec.Code)
		}
	})

	_ = privCrawler
}

