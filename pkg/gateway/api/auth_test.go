package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"

	oidcpkg "github.com/intentd/inrun/pkg/gateway/oidc"
	"github.com/intentd/inrun/pkg/types"
)

func TestTokenSet_Matches(t *testing.T) {
	ts := &TokenSet{
		entries: []resolvedToken{
			{name: "ci", value: "token-abc"},
			{name: "dev", value: "token-xyz"},
		},
	}

	if got := ts.Matches("token-abc"); got != "ci" {
		t.Errorf("Matches(token-abc) = %q, want %q", got, "ci")
	}
	if got := ts.Matches("token-xyz"); got != "dev" {
		t.Errorf("Matches(token-xyz) = %q, want %q", got, "dev")
	}
	if got := ts.Matches("unknown"); got != "" {
		t.Errorf("Matches(unknown) = %q, want empty", got)
	}
	if got := ts.Matches(""); got != "" {
		t.Errorf("Matches('') = %q, want empty", got)
	}
}

func TestAuthMiddleware_Valid(t *testing.T) {
	ts := &TokenSet{entries: []resolvedToken{{name: "ci", value: "secret-token"}}}
	called := false
	handler := AuthMiddleware(ts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if name := TokenNameFromContext(r.Context()); name != "ci" {
			t.Errorf("token name in ctx = %q, want %q", name, "ci")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestAuthMiddleware_Invalid(t *testing.T) {
	ts := &TokenSet{entries: []resolvedToken{{name: "ci", value: "secret-token"}}}
	handler := AuthMiddleware(ts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be called on invalid token")
	}))

	cases := []string{"Bearer wrong", "wrong", "", "Bearer "}
	for _, auth := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/apply", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("auth=%q: status = %d, want 401", auth, rr.Code)
		}
	}
}

func TestExpandEnvToken(t *testing.T) {
	t.Setenv("MY_TEST_TOKEN", "expanded-value")

	val, err := expandEnvToken("${MY_TEST_TOKEN}")
	if err != nil {
		t.Fatalf("expandEnvToken: %v", err)
	}
	if val != "expanded-value" {
		t.Errorf("got %q, want %q", val, "expanded-value")
	}

	_, err = expandEnvToken("literal-value")
	if err == nil {
		t.Error("expected error for literal value")
	}

	_, err = expandEnvToken("${UNSET_VAR_XYZ_ABC}")
	if err == nil {
		t.Error("expected error for unset env var")
	}
}

func TestGenerateUUIDv4(t *testing.T) {
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	u1, err := generateUUIDv4()
	if err != nil {
		t.Fatalf("generateUUIDv4: %v", err)
	}
	if !uuidRe.MatchString(u1) {
		t.Errorf("uuid %q does not match v4 format", u1)
	}

	u2, _ := generateUUIDv4()
	if u1 == u2 {
		t.Error("generateUUIDv4 produced duplicate value")
	}
}

func TestTokenNameFromContext_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if name := TokenNameFromContext(req.Context()); name != "" {
		t.Errorf("got %q, want empty", name)
	}
}

// ── OIDC helpers ──────────────────────────────────────────────────────────────

func oidcTestKey(t *testing.T, kid string) (*rsa.PrivateKey, jose.JSONWebKeySet) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := jose.JSONWebKey{Key: priv.Public(), KeyID: kid, Algorithm: string(jose.RS256), Use: "sig"}
	return priv, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
}

func oidcSignToken(t *testing.T, priv *rsa.PrivateKey, kid string, claims map[string]interface{}) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: priv},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := josejwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func oidcTestServer(t *testing.T, ks jose.JSONWebKeySet) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ks)
	})
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"jwks_uri": "http://" + r.Host + "/jwks",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ── OIDC TokenSet tests ───────────────────────────────────────────────────────

func TestTokenSet_MatchesOIDC_ValidToken(t *testing.T) {
	priv, ks := oidcTestKey(t, "k1")
	srv := oidcTestServer(t, ks)
	issuer := srv.URL

	token := oidcSignToken(t, priv, "k1", map[string]interface{}{
		"iss":        issuer,
		"sub":        "repo:myorg/payments:ref:refs/heads/main",
		"aud":        "inrun",
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
		"repository": "myorg/payments",
		"ref":        "refs/heads/main",
	})

	ts := &TokenSet{
		oidcCache: oidcpkg.NewCache(time.Minute),
		oidcEntries: []types.APIToken{{
			Name: "ci-oidc",
			OIDC: &types.OIDCToken{
				Issuer:   issuer,
				Audience: "inrun",
				Allow:    map[string]string{"repository": "myorg/payments", "ref": "refs/heads/main"},
			},
		}},
	}

	name, sub := ts.MatchesOIDC(context.Background(), token)
	if name != "ci-oidc" {
		t.Errorf("MatchesOIDC name = %q, want %q", name, "ci-oidc")
	}
	if sub != "repo:myorg/payments:ref:refs/heads/main" {
		t.Errorf("MatchesOIDC sub = %q, want %q", sub, "repo:myorg/payments:ref:refs/heads/main")
	}
}

func TestTokenSet_MatchesOIDC_ClaimMismatch(t *testing.T) {
	priv, ks := oidcTestKey(t, "k1")
	srv := oidcTestServer(t, ks)
	issuer := srv.URL

	// Token is from myorg/payments but token entry requires myorg/billing.
	token := oidcSignToken(t, priv, "k1", map[string]interface{}{
		"iss":        issuer,
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
		"repository": "myorg/payments",
	})

	ts := &TokenSet{
		oidcCache: oidcpkg.NewCache(time.Minute),
		oidcEntries: []types.APIToken{{
			Name: "billing-ci",
			OIDC: &types.OIDCToken{
				Issuer: issuer,
				Allow:  map[string]string{"repository": "myorg/billing"},
			},
		}},
	}

	if name, _ := ts.MatchesOIDC(context.Background(), token); name != "" {
		t.Errorf("MatchesOIDC = %q, want empty (claim mismatch)", name)
	}
}

func TestTokenSet_MatchesOIDC_ExpiredToken(t *testing.T) {
	priv, ks := oidcTestKey(t, "k1")
	srv := oidcTestServer(t, ks)
	issuer := srv.URL

	token := oidcSignToken(t, priv, "k1", map[string]interface{}{
		"iss": issuer,
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
	})

	ts := &TokenSet{
		oidcCache: oidcpkg.NewCache(time.Minute),
		oidcEntries: []types.APIToken{{
			Name: "ci-oidc",
			OIDC: &types.OIDCToken{Issuer: issuer},
		}},
	}

	if name, _ := ts.MatchesOIDC(context.Background(), token); name != "" {
		t.Errorf("MatchesOIDC = %q, want empty (expired)", name)
	}
}

func TestTokenSet_MatchesOIDC_NotAJWT(t *testing.T) {
	ts := &TokenSet{
		oidcCache: oidcpkg.NewCache(time.Minute),
		oidcEntries: []types.APIToken{{
			Name: "ci-oidc",
			OIDC: &types.OIDCToken{Issuer: "https://example.com"},
		}},
	}

	if name, _ := ts.MatchesOIDC(context.Background(), "not-a-jwt"); name != "" {
		t.Errorf("MatchesOIDC = %q, want empty (not a JWT)", name)
	}
}

func TestTokenSet_MatchesOIDC_NoEntries(t *testing.T) {
	ts := &TokenSet{oidcCache: oidcpkg.NewCache(time.Minute)}
	if name, _ := ts.MatchesOIDC(context.Background(), "any.token.value"); name != "" {
		t.Errorf("MatchesOIDC = %q, want empty (no entries)", name)
	}
}
