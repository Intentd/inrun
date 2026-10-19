// Package app assembles the Inrun gateway: the Serve layer (Gateway API),
// schema migration, everywhere, and inside a pod also TLS,
// the admission and conversion webhook server and its /blueprint routes.
// New wires everything and starts nothing; the caller starts the process.
// The gateway runs no reconcilers, informers or leader election:
// it is stateless and runs as several replicas.
package app

import (
	"context"
	"fmt"

	kubeutil "github.com/intentd/inrun/pkg/util/kube"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/certmanager"
	"github.com/intentd/inrun/pkg/gateway/webhook"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/process"
)

// Gateway is the assembled gateway. Start it with Process.Start.
type Gateway struct {
	Blueprint *blueprint.Blueprint
	Process   *process.Manager
}

// GatewayOptions controls optional behavior of the assembled Gateway.
type GatewayOptions struct {
	// Migrate runs the Blueprint's declared schema migrations against live CRs
	// during startup, before the gateway begins serving. When false, the
	// gateway assumes CRs are already in the current schema.
	Migrate bool
}

// New wires the gateway. Inside a pod it serves the webhooks over TLS next to
// the Gateway API. Outside one there is no cluster to hold certificates or
// patch CRDs, so it serves the Gateway API (POST /api/v1/apply,
// GET /api/v1/resources/, and so on) on the plain
// HTTP health port — the same API surface — and leaves the webhooks out.
func New(ctx context.Context, cfg *config.Config, m *merger.Merger, opts GatewayOptions) (*Gateway, error) {
	bp, kube, err := buildBase(ctx, cfg, m)
	if err != nil {
		return nil, err
	}

	// Apply schema migrations if any
	if opts.Migrate {
		if err := migrate.Apply(ctx, bp, kube); err != nil {
			return nil, fmt.Errorf("schema migration failed: %w", err)
		}
	}

	// HTTP: probes, metrics, the /blueprint API and the Gateway API.
	hs := health.NewHealthServer(cfg)

	if !kubeutil.IsRunningInPod() {
		logger.Warn().Msg("not in a pod — admission and conversion webhooks are disabled (TLS not available)")

		api, err := newAPI(ctx, cfg, bp, kube)
		if err != nil {
			return nil, err
		}

		if api != nil {
			api.Register(hs)
		}
		return &Gateway{Blueprint: bp, Process: process.New(cfg, []domain.Component{hs, kube})}, nil
	}

	// Namespace label, TLS and the conversion caBundle.
	tls, err := ensureSecurity(ctx, cfg, bp, kube)
	if err != nil {
		return nil, fmt.Errorf("security setup failed: %w", err)
	}
	if tls != nil {
		logger.Debug().
			Str("cert_file", tls.certFile).
			Str("cert_key", tls.keyFile).
			Msg("passing TLS cert to webhook server")
		cfg.Security().Webhooks.TLSCert = tls.certFile
		cfg.Security().Webhooks.TLSKey = tls.keyFile
	}

	// HTTPS: /validate, /mutate, /convert.
	ws := webhook.NewWebhookServer(kube.Clientset(), bp, cfg)
	if tls != nil && tls.bundle != nil {
		ws.SetCertManager(tls.mgr)
		ws.SetCertBundle(tls.bundle.CertPEM, tls.bundle.KeyPEM, tls.bundle.CACertPEM,
			certmanager.DefaultTLSSecretName, cfg.Cluster().Namespace())
	}
	// Keeps CRD conversion caBundles correct throughout the deployment
	// lifecycle.
	wireConversion(ws, kube, bp, cfg)

	registerBlueprintRoutes(hs, bp, ws)

	api, err := newAPI(ctx, cfg, bp, kube)
	if err != nil {
		return nil, err
	}

	if api != nil {
		api.Register(hs)
		ws.SetTokenReloader(api.ReloadTokens)
	}

	return &Gateway{Blueprint: bp, Process: process.New(cfg, []domain.Component{
		hs,   // 1. HTTP server — /ready, /livez probes
		ws,   // 2. HTTPS webhook server — /validate, /mutate, /convert
		kube, // 3. REST clients — already started, managed for Stop()
	})}, nil
}
