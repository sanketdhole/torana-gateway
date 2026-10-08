package agentauth_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"github.com/phaselume/torana/internal/agentauth"
)

// mockRevocations implements agentauth.RevocationChecker for testing.
type mockRevocations struct {
	revoked map[string]bool
}

func (m *mockRevocations) IsTokenRevoked(id string) bool {
	return m.revoked[id]
}

func (m *mockRevocations) IsKeyRevoked(id string) bool {
	return m.revoked[id]
}

func TestPrincipalURIParsing(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		wantType  agentauth.PrincipalType
		wantOrg   string
		wantNs    string
		wantID    string
		expectErr bool
	}{
		{
			name:      "valid human user",
			uri:       "torana://org_123/prod/user/usr_sanket",
			wantType:  agentauth.PrincipalTypeUser,
			wantOrg:   "org_123",
			wantNs:    "prod",
			wantID:    "usr_sanket",
			expectErr: false,
		},
		{
			name:      "valid agent",
			uri:       "torana://org_123/prod/agent/agt_planner",
			wantType:  agentauth.PrincipalTypeAgent,
			wantOrg:   "org_123",
			wantNs:    "prod",
			wantID:    "agt_planner",
			expectErr: false,
		},
		{
			name:      "valid tool",
			uri:       "torana://org_123/prod/tool/tool_db_query",
			wantType:  agentauth.PrincipalTypeTool,
			wantOrg:   "org_123",
			wantNs:    "prod",
			wantID:    "tool_db_query",
			expectErr: false,
		},
		{
			name:      "valid system process",
			uri:       "torana://org_123/prod/system/sys_sync",
			wantType:  agentauth.PrincipalTypeSystem,
			wantOrg:   "org_123",
			wantNs:    "prod",
			wantID:    "sys_sync",
			expectErr: false,
		},
		{
			name:      "invalid scheme",
			uri:       "http://org_123/prod/user/usr_1",
			expectErr: true,
		},
		{
			name:      "missing segment",
			uri:       "torana://org_123/prod/user",
			expectErr: true,
		},
		{
			name:      "unrecognized principal type",
			uri:       "torana://org_123/prod/robot/bot_1",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := agentauth.ParsePrincipalURI(tt.uri)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.uri)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.uri, err)
			}
			if p.Type != tt.wantType || p.OrgID != tt.wantOrg || p.Namespace != tt.wantNs || p.ID != tt.wantID {
				t.Errorf("parsed principal mismatch: got %+v", p)
			}
		})
	}
}

func TestScopeAttenuation(t *testing.T) {
	tests := []struct {
		name       string
		parent     []string
		child      []string
		expectPass bool
	}{
		{
			name:       "exact match",
			parent:     []string{"db:read", "analytics:query"},
			child:      []string{"db:read"},
			expectPass: true,
		},
		{
			name:       "wildcard asterisk grants everything",
			parent:     []string{"*"},
			child:      []string{"admin:delete", "db:write"},
			expectPass: true,
		},
		{
			name:       "prefix wildcard attenuation",
			parent:     []string{"db:*", "analytics:*"},
			child:      []string{"db:read", "db:write", "analytics:export"},
			expectPass: true,
		},
		{
			name:       "unauthorized privilege escalation attempt",
			parent:     []string{"db:read"},
			child:      []string{"db:read", "db:write"},
			expectPass: false,
		},
		{
			name:       "prefix boundary check: db:* must not grant db_other:read",
			parent:     []string{"db:*"},
			child:      []string{"db_other:read"},
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, offending := agentauth.ScopesCover(tt.parent, tt.child)
			if ok != tt.expectPass {
				t.Errorf("ScopesCover(%v, %v) = %v (offending: %q), want %v", tt.parent, tt.child, ok, offending, tt.expectPass)
			}
		})
	}
}

func TestVerifier_MultiHopChain(t *testing.T) {
	// 1. Generate keys
	tenantPub, tenantPriv, _ := ed25519.GenerateKey(rand.Reader)
	agent1Pub, agent1Priv, _ := ed25519.GenerateKey(rand.Reader)
	agent2Pub, _, _ := ed25519.GenerateKey(rand.Reader)

	// 2. Setup registry
	registry := agentauth.NewRegistry()
	registry.SetTenantPublicKey(tenantPub)

	agent1URI := "torana://org_1/prod/agent/agt_planner"
	agent2URI := "torana://org_1/prod/agent/agt_executor"
	rootUserURI := "torana://org_1/prod/user/usr_sanket"
	targetToolURI := "torana://org_1/prod/tool/tool_db_query"

	_ = registry.RegisterAgent(agentauth.AgentRegistryEntry{
		AgentURI:      agent1URI,
		PublicKeyHex:  hex.EncodeToString(agent1Pub),
		RiskTier:      "LOW",
		AllowedScopes: []string{"db:*", "analytics:*"},
		Active:        true,
	})
	_ = registry.RegisterAgent(agentauth.AgentRegistryEntry{
		AgentURI:      agent2URI,
		PublicKeyHex:  hex.EncodeToString(agent2Pub),
		RiskTier:      "MEDIUM",
		AllowedScopes: []string{"db:read"},
		Active:        true,
	})

	revocations := &mockRevocations{revoked: make(map[string]bool)}
	verifier := agentauth.NewVerifier(registry, revocations)

	baseTime := time.Now().Unix()

	// Helper to create valid 2-hop token: User -> Agent1 -> Agent2
	createValidToken := func() *agentauth.ToranaDelegationToken {
		root := agentauth.DelegationRoot{
			Principal:     rootUserURI,
			IssuedAt:      baseTime - 10,
			ExpiresAt:     baseTime + 3600,
			InitialScopes: []string{"db:*", "analytics:*", "tools:execute"},
		}
		_ = agentauth.SignRoot(tenantPriv, &root)

		hop1 := agentauth.DelegationHop{
			HopIndex:         1,
			DelegatorURI:     rootUserURI,
			DelegateeURI:     agent1URI,
			AttenuatedScopes: []string{"db:*", "tools:execute"},
			IssuedAt:         baseTime - 5,
			ExpiresAt:        baseTime + 3000,
			Constraints:      map[string]string{"max_downstream_hops": "2"},
		}
		_ = agentauth.SignHop(tenantPriv, &hop1, root.SignatureHex)

		hop2 := agentauth.DelegationHop{
			HopIndex:         2,
			DelegatorURI:     agent1URI,
			DelegateeURI:     agent2URI,
			AttenuatedScopes: []string{"db:read", "tools:execute"},
			IssuedAt:         baseTime - 2,
			ExpiresAt:        baseTime + 2000,
		}
		_ = agentauth.SignHop(agent1Priv, &hop2, hop1.SignatureHex)

		return &agentauth.ToranaDelegationToken{
			Version: "1.0",
			Root:    root,
			Hops:    []agentauth.DelegationHop{hop1, hop2},
		}
	}

	t.Run("valid multi-hop delegation chain verifies successfully", func(t *testing.T) {
		tok := createValidToken()
		res := verifier.Verify(context.Background(), tok)
		if !res.Valid {
			t.Fatalf("expected valid verification, got error: %s", res.Reason)
		}
		if res.RootPrincipal != rootUserURI {
			t.Errorf("expected root principal %s, got %s", rootUserURI, res.RootPrincipal)
		}
		if res.CallerPrincipal != agent2URI {
			t.Errorf("expected caller principal %s, got %s", agent2URI, res.CallerPrincipal)
		}
		if res.ChainDepth != 2 {
			t.Errorf("expected chain depth 2, got %d", res.ChainDepth)
		}
		if len(res.EffectiveScopes) != 2 || res.EffectiveScopes[0] != "db:read" {
			t.Errorf("unexpected effective scopes: %v", res.EffectiveScopes)
		}
	})

	t.Run("tampered root signature fails verification", func(t *testing.T) {
		tok := createValidToken()
		tok.Root.SignatureHex = hex.EncodeToString([]byte("bad_signature_bytes_length_64_padding_to_exact_ed25519_size_test"))
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on tampered root signature")
		}
		if res.Reason != "invalid_root_signature" {
			t.Errorf("unexpected reason: %s", res.Reason)
		}
	})

	t.Run("tampered hop signature fails verification", func(t *testing.T) {
		tok := createValidToken()
		tok.Hops[1].SignatureHex = hex.EncodeToString([]byte("tampered_hop_sig_padding_bytes_32_characters_total_length_ed25519_ok"))
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on tampered hop signature")
		}
		if res.Reason != "invalid_hop_signature: hop 2" {
			t.Errorf("unexpected reason: %s", res.Reason)
		}
	})

	t.Run("scope escalation fails verification", func(t *testing.T) {
		tok := createValidToken()
		// Hop 2 attempts privilege escalation to admin:delete
		tok.Hops[1].AttenuatedScopes = []string{"admin:delete"}
		_ = agentauth.SignHop(agent1Priv, &tok.Hops[1], tok.Hops[0].SignatureHex)
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on scope escalation")
		}
		if res.StatusCode != 403 {
			t.Errorf("expected status 403, got %d", res.StatusCode)
		}
	})

	t.Run("expired root token fails verification", func(t *testing.T) {
		tok := createValidToken()
		tok.Root.ExpiresAt = baseTime - 100 // expired
		_ = agentauth.SignRoot(tenantPriv, &tok.Root)
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on expired root token")
		}
		if res.Reason != "root_token_expired" {
			t.Errorf("unexpected reason: %s", res.Reason)
		}
	})

	t.Run("expired hop fails verification", func(t *testing.T) {
		tok := createValidToken()
		tok.Hops[1].ExpiresAt = baseTime - 50 // expired
		_ = agentauth.SignHop(agent1Priv, &tok.Hops[1], tok.Hops[0].SignatureHex)
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on expired hop")
		}
		if res.StatusCode != 401 {
			t.Errorf("expected status 401, got %d", res.StatusCode)
		}
	})

	t.Run("TTL decay violation (child expires after parent)", func(t *testing.T) {
		tok := createValidToken()
		tok.Hops[1].ExpiresAt = tok.Hops[0].ExpiresAt + 500 // decays must not increase
		_ = agentauth.SignHop(agent1Priv, &tok.Hops[1], tok.Hops[0].SignatureHex)
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on TTL decay violation")
		}
	})

	t.Run("max_downstream_hops exceeded", func(t *testing.T) {
		tok := createValidToken()
		// Hop 1 specifies max_downstream_hops = 0, but hop 2 exists downstream
		tok.Hops[0].Constraints = map[string]string{"max_downstream_hops": "0"}
		_ = agentauth.SignHop(tenantPriv, &tok.Hops[0], tok.Root.SignatureHex)
		// re-sign hop 2 with new hop 1 sig
		_ = agentauth.SignHop(agent1Priv, &tok.Hops[1], tok.Hops[0].SignatureHex)

		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure when max_downstream_hops is exceeded")
		}
		if res.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden, got %d", res.StatusCode)
		}
	})

	t.Run("revoked delegatee fails verification", func(t *testing.T) {
		tok := createValidToken()
		revocations.revoked[agent2URI] = true
		defer func() { revocations.revoked[agent2URI] = false }()

		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on revoked agent delegatee")
		}
		if res.StatusCode != 403 {
			t.Errorf("expected 403 Forbidden, got %d", res.StatusCode)
		}
	})

	t.Run("broken chain continuity fails verification", func(t *testing.T) {
		tok := createValidToken()
		tok.Hops[1].DelegatorURI = "torana://org_1/prod/agent/unrelated_agent"
		_ = agentauth.SignHop(agent1Priv, &tok.Hops[1], tok.Hops[0].SignatureHex)
		res := verifier.Verify(context.Background(), tok)
		if res.Valid {
			t.Fatal("expected failure on broken chain continuity")
		}
	})

	_ = targetToolURI
}

func TestCELPolicyEvaluation_WithChainVariables(t *testing.T) {
	// Test CEL expression from the user requirement:
	// chain.root.type == "user" &&
	// chain.depth <= 2 &&
	// "tools:execute" in chain.effective_scopes &&
	// !chain.hops.exists(h, h.delegatee.endsWith("/unverified_crawler"))

	env, err := cel.NewEnv(
		cel.Variable("chain", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		t.Fatalf("failed to create CEL env: %v", err)
	}

	expr := `chain.root.type == "user" &&
chain.depth <= 2 &&
"tools:execute" in chain.effective_scopes &&
!chain.hops.exists(h, h.delegatee.endsWith("/unverified_crawler"))`

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		t.Fatalf("failed to compile CEL expression: %v", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		t.Fatalf("failed to create CEL program: %v", err)
	}

	// 1. Successful evaluation case
	validChain := &agentauth.ChainContext{
		RootPrincipal:   "torana://org_1/prod/user/usr_sanket",
		RootType:        "user",
		Depth:           2,
		EffectiveScopes: []string{"db:read", "tools:execute"},
		CallerURI:       "torana://org_1/prod/agent/agt_executor",
		Hops: []agentauth.ChainHopContext{
			{
				Delegator: "torana://org_1/prod/user/usr_sanket",
				Delegatee: "torana://org_1/prod/agent/agt_planner",
				Scopes:    []string{"db:*", "tools:execute"},
			},
			{
				Delegator: "torana://org_1/prod/agent/agt_planner",
				Delegatee: "torana://org_1/prod/agent/agt_executor",
				Scopes:    []string{"db:read", "tools:execute"},
			},
		},
	}

	out, _, err := prg.Eval(map[string]any{
		"chain": agentauth.BuildCELChainMap(validChain),
	})
	if err != nil {
		t.Fatalf("CEL evaluation failed: %v", err)
	}
	if out.Value() != true {
		t.Errorf("expected CEL rule to evaluate to true, got %v", out.Value())
	}

	// 2. Failure case: Unverified crawler in chain
	crawlerChain := &agentauth.ChainContext{
		RootPrincipal:   "torana://org_1/prod/user/usr_sanket",
		RootType:        "user",
		Depth:           2,
		EffectiveScopes: []string{"tools:execute"},
		CallerURI:       "torana://org_1/prod/agent/agt_executor",
		Hops: []agentauth.ChainHopContext{
			{
				Delegator: "torana://org_1/prod/user/usr_sanket",
				Delegatee: "torana://org_1/prod/agent/unverified_crawler",
				Scopes:    []string{"tools:execute"},
			},
			{
				Delegator: "torana://org_1/prod/agent/unverified_crawler",
				Delegatee: "torana://org_1/prod/agent/agt_executor",
				Scopes:    []string{"tools:execute"},
			},
		},
	}

	out2, _, err := prg.Eval(map[string]any{
		"chain": agentauth.BuildCELChainMap(crawlerChain),
	})
	if err != nil {
		t.Fatalf("CEL evaluation failed: %v", err)
	}
	if out2.Value() != false {
		t.Errorf("expected crawler chain to evaluate to false, got %v", out2.Value())
	}

	// 3. Failure case: Depth exceeded (> 2)
	deepChain := &agentauth.ChainContext{
		RootPrincipal:   "torana://org_1/prod/user/usr_sanket",
		RootType:        "user",
		Depth:           3,
		EffectiveScopes: []string{"tools:execute"},
		CallerURI:       "torana://org_1/prod/agent/agt_3",
		Hops:            []agentauth.ChainHopContext{},
	}

	out3, _, err := prg.Eval(map[string]any{
		"chain": agentauth.BuildCELChainMap(deepChain),
	})
	if err != nil {
		t.Fatalf("CEL evaluation failed: %v", err)
	}
	if out3.Value() != false {
		t.Errorf("expected deep chain to evaluate to false, got %v", out3.Value())
	}
}
