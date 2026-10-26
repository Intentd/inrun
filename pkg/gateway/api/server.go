// pkg/gateway/api/server.go
//
// APIServer wires the Gateway API handlers onto a health.HealthServer.
// Called from cmd/internal/gateway.go after the kubeclient is ready.
//
// Route layout:
//
//			POST   /api/v1/apply                          → applyHandler
//		 	GET    /api/v1/resources/{kind}/{ns}          → resourcesHandler — list
//			GET    /api/v1/resources/{kind}/{ns}[/{name}] → resourcesHandler — get
//			DELETE /api/v1/resources/{kind}/{ns}/{name}   → resourcesHandler — delete
//			GET    /api/v1/schema/                        → schemaHandler — (service list — serve-enabled CRDs)
//		 	GET    /api/v1/schema?target=<t>              → schemaHandler — schema for target
//	  		GET    /api/v1/raw-schema?kind=<k>            → schemaHandler — raw Kubernetes OpenAPI spec schema
//
// All routes are wrapped by AuthMiddleware before registration.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/intentd/inrun/pkg/blueprint"
	oidcpkg "github.com/intentd/inrun/pkg/gateway/oidc"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
)

// Registrar is the subset of health.HealthServer used here.
// Defined as an interface so the server package does not import pkg/health.
type Registrar interface {
	Register(path string, handler http.HandlerFunc)
}

// APIServer holds the resolved token set and wired handlers.
type APIServer struct {
	mu        sync.RWMutex
	tokens    *TokenSet
	oidcCache *oidcpkg.Cache
	kube      kubeclient.Interface
	clusters  *ClusterRegistry
	bp        *blueprint.Blueprint
	ownNS     string
}

// NewAPIServer resolves all tokens (bootstrapping/rotating Secrets as
// needed) and returns a ready-to-register server.
func NewAPIServer(ctx context.Context, bp *blueprint.Blueprint, kube kubeclient.Interface, clusters *ClusterRegistry, ownNS string) (*APIServer, error) {
	if !bp.HasServeEnabled() {
		return nil, nil // not enabled — caller skips registration
	}

	cache := oidcpkg.NewCache(oidcpkg.DefaultTTL)
	tokens, err := LoadTokens(ctx, bp.Gateway.API.Auth.Tokens, kube, ownNS, cache)
	if err != nil {
		return nil, fmt.Errorf("loading Gateway API tokens: %w", err)
	}

	return &APIServer{
		tokens:    tokens,
		oidcCache: cache,
		kube:      kube,
		clusters:  clusters,
		bp:        bp,
		ownNS:     ownNS,
	}, nil
}

// matches returns the token name for value, reading through the reload mutex.
func (s *APIServer) matches(value string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokens.Matches(value)
}

// ReloadTokens re-resolves all secretRef tokens (recreating any missing secrets)
// and atomically replaces the in-memory TokenSet. The OIDC cache is reused —
// cached JWKS keys persist across reloads.
func (s *APIServer) ReloadTokens(ctx context.Context) error {
	ts, err := LoadTokens(ctx, s.bp.Gateway.API.Auth.Tokens, s.kube, s.ownNS, s.oidcCache)
	if err != nil {
		return fmt.Errorf("reload Gateway API tokens: %w", err)
	}
	s.mu.Lock()
	s.tokens = ts
	s.mu.Unlock()
	return nil
}

// Register wires all Gateway API routes onto the given Registrar.
func (s *APIServer) Register(reg Registrar) {
	if !s.bp.HasServeEnabled() {
		return
	}

	// auth closes over s so that token lookups always read the current TokenSet
	// even after a ReloadTokens call swaps the pointer.
	// Static bearer tokens are checked first (O(n), in-process). OIDC JWTs are
	// checked second — they require JWKS cache lookup and JWT verification.
	auth := func(h http.Handler) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearer := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			name := s.matches(bearer)
			if name == "" {
				writeJSONError(w, http.StatusUnauthorized, "Unauthorized", "invalid token")
				return
			}
			ctx := contextWithTokenName(r.Context(), name)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// POST /api/v1/apply
	reg.Register("/api/v1/apply", auth(applyHandler(s.kube, s.clusters, s.bp)))

	// GET/DELETE /api/v1/resources/...
	reg.Register("/api/v1/resources/", auth(resourcesHandler(s.kube, s.clusters, s.bp)))

	// GET /api/v1/schema/...
	reg.Register("/api/v1/schema", auth(schemaHandler(s.bp)))

	// GET /api/v1/raw-schema
	reg.Register("/api/v1/raw-schema", auth(rawSchemaHandler(s.kube, s.clusters, s.bp)))

	logger.Info().Msg("Gateway API routes registered: /api/v1/apply, /api/v1/resources/, /api/v1/schema, /api/v1/raw-schema")
}
