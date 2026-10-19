// pkg/gateway/auth.go
//
// Bearer token authentication for the Gateway API.
//
// At startup, LoadTokens resolves every token entry in the Blueprint config:
//   - secretRef entries: read or self-bootstrap the Kubernetes Secret, then
//     optionally rotate it if rotateAfter has elapsed.
//   - token entries: expand ${ENV_VAR} references.
//   - githubOIDC / gitlabOIDC / oidc entries: stored as-is; verified per request
//     via the shared JWKS cache in pkg/gateway/oidc.
//
// The resolved values are stored in TokenSet, which the auth closure uses
// to validate incoming Authorization: Bearer <token> headers.
//
// Self-bootstrap and rotation reuse pkg/secrets helpers so the
// annotation-based rotation clock is identical to operator-managed secrets.
package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	oidcpkg "github.com/intentd/inrun/pkg/gateway/oidc"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/secrets"
	"github.com/intentd/inrun/pkg/types"
)

// TokenSet holds resolved static bearer tokens and OIDC token configurations.
// Lookups are O(n) — Gateway API token counts are tiny.
type TokenSet struct {
	entries     []resolvedToken
	oidcEntries []types.APIToken
	oidcCache   *oidcpkg.Cache
}

type resolvedToken struct {
	name  string
	value string
}

// Matches returns the token name if value matches a static bearer token.
func (ts *TokenSet) Matches(value string) string {
	for _, e := range ts.entries {
		if e.value == value {
			return e.name
		}
	}
	return ""
}

// MatchesOIDC returns the token name and verified sub claim if the bearer value
// is a valid JWT that satisfies one of the configured OIDC token entries.
// Returns ("", "") on any mismatch or verification failure.
func (ts *TokenSet) MatchesOIDC(ctx context.Context, bearer string) (name, sub string) {
	if ts.oidcCache == nil || len(ts.oidcEntries) == 0 {
		return "", ""
	}
	iss, ok := oidcpkg.IssuerFromToken(bearer)
	if !ok {
		return "", ""
	}
	for _, entry := range ts.oidcEntries {
		if entry.OIDCIssuer() != iss {
			continue
		}
		claims, err := ts.oidcCache.Verify(iss, entry.OIDCDiscoveryBase(), bearer, entry.OIDCAudience())
		if err != nil {
			logger.FromContext(ctx).Debug().
				Err(err).
				Str("token", entry.Name).
				Str("kind", entry.OIDCKind()).
				Msg("gateway: OIDC verification failed")
			continue
		}
		if entry.MatchesOIDCClaims(claims) {
			return entry.Name, claims["sub"]
		}
	}
	return "", ""
}

// LoadTokens resolves all Gateway API token entries from the Blueprint config.
// secretRef entries are bootstrapped/rotated via the Kubernetes client.
// token entries are expanded from environment variables.
// OIDC entries are stored in the TokenSet and verified per request via oidcCache.
func LoadTokens(ctx context.Context, tokens []types.APIToken, kube kubeclient.Interface, ownNamespace string, oidcCache *oidcpkg.Cache) (*TokenSet, error) {
	ts := &TokenSet{oidcCache: oidcCache}
	for _, t := range tokens {
		switch {
		case t.SecretRef != nil:
			val, err := ResolveSecretRef(ctx, t.SecretRef, kube, ownNamespace)
			if err != nil {
				return nil, fmt.Errorf("token %q: %w", t.Name, err)
			}
			ts.entries = append(ts.entries, resolvedToken{name: t.Name, value: val})

		case t.Token != "":
			val, err := expandEnvToken(t.Token)
			if err != nil {
				return nil, fmt.Errorf("token %q: %w", t.Name, err)
			}
			ts.entries = append(ts.entries, resolvedToken{name: t.Name, value: val})

		case t.IsOIDC():
			ts.oidcEntries = append(ts.oidcEntries, t)

		default:
			return nil, fmt.Errorf("token %q: must set one of: token, secretRef, githubOIDC, gitlabOIDC, oidc", t.Name)
		}
	}
	return ts, nil
}

// ResolveSecretRef reads the token from a Kubernetes Secret, creating or
// rotating it as needed using the same annotation-based rotation as pkg/secrets.
func ResolveSecretRef(ctx context.Context, ref *types.APISecretRef, kube kubeclient.Interface, ownNamespace string) (string, error) {
	ns := ref.Namespace
	if ns == "" {
		ns = ownNamespace
	}

	// Rotation check: if the Secret exists and rotateAfter has elapsed, delete it
	// so the create step below regenerates it with a fresh token.
	if ref.RotateAfter != "" {
		needsRotation, err := secrets.SecretNeedsRotation(ctx, kube, ns, ref.Name, ref.RotateAfter)
		if err != nil {
			return "", fmt.Errorf("checking rotation for %s/%s: %w", ns, ref.Name, err)
		}
		if needsRotation {
			logger.FromContext(ctx).Info().
				Str("secret", ref.Name).
				Str("namespace", ns).
				Str("rotateAfter", ref.RotateAfter).
				Msg("gateway API token rotation: deleting expired secret")
			if err := secrets.DeleteSecretForRotation(ctx, kube, ns, ref.Name); err != nil {
				return "", fmt.Errorf("rotating %s/%s: %w", ns, ref.Name, err)
			}
		}
	}

	// Read or bootstrap: check existence, create if missing.
	exists, err := secrets.SecretExists(ctx, kube, ns, ref.Name)
	if err != nil {
		return "", fmt.Errorf("checking secret %s/%s: %w", ns, ref.Name, err)
	}
	if !exists {
		token, err := generateUUIDv4()
		if err != nil {
			return "", fmt.Errorf("generating token for %s/%s: %w", ns, ref.Name, err)
		}
		if err := createTokenSecret(ctx, kube, ns, ref.Name, ref.Key, token, ref.RotateAfter); err != nil {
			return "", fmt.Errorf("creating secret %s/%s: %w", ns, ref.Name, err)
		}
		logger.FromContext(ctx).Info().
			Str("secret", ref.Name).
			Str("namespace", ns).
			Msg("gateway API token: bootstrapped new secret")
		return token, nil
	}

	// Secret exists — read the token value.
	return secrets.ReadSecretKey(ctx, kube.Clientset(), ns, ref.Name, ref.Key)
}

// createTokenSecret creates a new Kubernetes Secret with the given token value
// and generation annotations for the rotation clock.
func createTokenSecret(ctx context.Context, kube kubeclient.Interface, ns, name, key, token, rotateAfter string) error {
	annotations := secrets.GenerationAnnotations(rotateAfter)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels.WithDeletionProtection(labels.InrunResourceLabels()),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			key: token,
		},
	}
	_, err := kube.Clientset().CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// expandEnvToken resolves ${ENV_VAR} or $ENV_VAR references.
// Returns an error for empty values or literal non-variable strings.
func expandEnvToken(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "$") {
		return "", fmt.Errorf("token value must be an ${ENV_VAR} reference, got literal: %q", trimmed)
	}
	expanded := os.ExpandEnv(trimmed)
	if expanded == "" {
		return "", fmt.Errorf("env var %q is not set or empty", trimmed)
	}
	return expanded, nil
}

// generateUUIDv4 generates a random UUID v4 string in 8-4-4-4-12 hex format.
// Same implementation as the uuidv4 note in pkg/note/random.go.
func generateUUIDv4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("uuidv4: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// AuthMiddleware returns an http.Handler that validates the Authorization:
// Bearer header against the TokenSet. On match, the token name is added to
// the request context. On mismatch, responds 401.
func AuthMiddleware(ts *TokenSet, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		bearer = strings.TrimSpace(bearer)
		name := ts.Matches(bearer)
		if name == "" {
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized", "invalid token")
			return
		}
		ctx := contextWithTokenName(r.Context(), name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextKey string

const tokenNameKey contextKey = "api-token-name"

func contextWithTokenName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, tokenNameKey, name)
}

// TokenNameFromContext returns the authenticated token name from the context.
// Empty string when the request was not authenticated (should not happen past middleware).
func TokenNameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tokenNameKey).(string)
	return v
}
