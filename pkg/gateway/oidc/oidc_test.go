package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"

	oidcpkg "github.com/intentd/inrun/pkg/gateway/oidc"
)

// testKey generates an RSA key pair and returns the private key + a single-key JWKS.
func testKey(t *testing.T, kid string) (*rsa.PrivateKey, jose.JSONWebKeySet) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := jose.JSONWebKey{Key: priv.Public(), KeyID: kid, Algorithm: string(jose.RS256), Use: "sig"}
	return priv, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
}

// signToken signs a JWT with the given private key and claims.
func signToken(t *testing.T, priv *rsa.PrivateKey, kid string, claims map[string]interface{}) string {
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

// testServer returns an httptest.Server that serves the JWKS at /jwks and
// an OIDC discovery doc at /.well-known/openid-configuration.
func testServer(t *testing.T, ks jose.JSONWebKeySet) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ks)
	})
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// Discovery doc — dynamically set jwks_uri to match server address.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"jwks_uri": "http://" + r.Host + "/jwks",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestVerify_ValidToken(t *testing.T) {
	priv, ks := testKey(t, "key1")
	srv := testServer(t, ks)

	issuer := srv.URL
	now := time.Now()
	token := signToken(t, priv, "key1", map[string]interface{}{
		"iss":        issuer,
		"sub":        "user:test",
		"aud":        "inrun",
		"exp":        now.Add(5 * time.Minute).Unix(),
		"repository": "myorg/payments",
		"ref":        "refs/heads/main",
	})

	cache := oidcpkg.NewCache(time.Minute)
	claims, err := cache.Verify(issuer, issuer, token, "inrun")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if claims["repository"] != "myorg/payments" {
		t.Errorf("repository claim: got %q, want %q", claims["repository"], "myorg/payments")
	}
	if claims["ref"] != "refs/heads/main" {
		t.Errorf("ref claim: got %q, want %q", claims["ref"], "refs/heads/main")
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	priv, ks := testKey(t, "key1")
	srv := testServer(t, ks)

	issuer := srv.URL
	token := signToken(t, priv, "key1", map[string]interface{}{
		"iss": issuer,
		"aud": "inrun",
		"exp": time.Now().Add(-1 * time.Minute).Unix(), // already expired
	})

	cache := oidcpkg.NewCache(time.Minute)
	_, err := cache.Verify(issuer, issuer, token, "inrun")
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	priv, ks := testKey(t, "key1")
	srv := testServer(t, ks)

	token := signToken(t, priv, "key1", map[string]interface{}{
		"iss": "https://evil.example.com", // wrong issuer
		"aud": "inrun",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	cache := oidcpkg.NewCache(time.Minute)
	_, err := cache.Verify(srv.URL, srv.URL, token, "inrun")
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}

func TestVerify_WrongAudience(t *testing.T) {
	priv, ks := testKey(t, "key1")
	srv := testServer(t, ks)

	issuer := srv.URL
	token := signToken(t, priv, "key1", map[string]interface{}{
		"iss": issuer,
		"aud": "other-service", // wrong audience
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	cache := oidcpkg.NewCache(time.Minute)
	_, err := cache.Verify(issuer, issuer, token, "inrun")
	if err == nil {
		t.Fatal("expected error for wrong audience, got nil")
	}
}

func TestVerify_UnknownKey(t *testing.T) {
	priv, _ := testKey(t, "key1")
	_, otherKS := testKey(t, "key2") // JWKS has key2, token is signed with key1
	srv := testServer(t, otherKS)

	issuer := srv.URL
	token := signToken(t, priv, "key1", map[string]interface{}{
		"iss": issuer,
		"aud": "inrun",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	cache := oidcpkg.NewCache(time.Minute)
	_, err := cache.Verify(issuer, issuer, token, "inrun")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestVerify_CachesJWKS(t *testing.T) {
	priv, ks := testKey(t, "key1")
	fetchCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		json.NewEncoder(w).Encode(ks)
	})
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"jwks_uri": "http://" + r.Host + "/jwks"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	issuer := srv.URL
	mkToken := func() string {
		return signToken(t, priv, "key1", map[string]interface{}{
			"iss": issuer,
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
	}

	cache := oidcpkg.NewCache(time.Minute)
	cache.Verify(issuer, issuer, mkToken(), "")
	cache.Verify(issuer, issuer, mkToken(), "")
	cache.Verify(issuer, issuer, mkToken(), "")

	if fetchCount != 1 {
		t.Errorf("expected JWKS fetched once, got %d fetches", fetchCount)
	}
}
