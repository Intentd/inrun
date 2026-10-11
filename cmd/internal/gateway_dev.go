// cmd/internal/gateway_dev.go
//
// Local gateway — HTTP-only variant for development. Handles the Serve layer
// (Gateway API + intake webhooks) without TLS or a live cluster. Admission and
// conversion webhooks are skipped; a warning is printed at startup.
//
// Included in dev builds (make inrun). Excluded from production gateway builds.

//go:build !runtime && !gateway

package internal

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/config"
	apigateway "github.com/intentd/inrun/pkg/gateway/api"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/process"
	"github.com/intentd/inrun/pkg/utils"
)

// RunGatewayDev starts a local HTTP-only gateway.
//
// TLS, WebhookServer, and /catalog routes that depend on webhook state are all
// omitted. The Gateway API (POST /api/v1/apply, GET /api/v1/resources/, etc.)
// and intake webhooks run on the plain HTTP health port — identical to the
// in-cluster API surface, with no certificate setup required.
func RunGatewayDev(kfg *config.Config, m *merger.Merger, ctx context.Context) {

	// ── 1. Instance + Catalog ─────────────────────────────────────────────────
	kfg.SetInstance(config.Gateway())

	kat := pipeline.NewCatalog(kfg, m)

	if registryURL := kfg.RegistryConfig().RegistryURL; registryURL != "" {
		m.SetRegistryURL(registryURL)
		logger.Info().Str("registry", registryURL).Msg("registry URL configured from INRUN_REGISTRY")
	}

	// ── 2. Scheme + Kubeclient ────────────────────────────────────────────────
	scheme, err := catalog.NewSchemeRegistry(kat)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build scheme registry")
	}

	kube := kubeclient.NewKubeclient(kfg, scheme)
	if err := kube.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start kubeclient")
	}

	// ── 3. HealthServer — HTTP ────────────────────────────────────────────────
	hs := health.NewHealthServer(kfg)

	// ── 4. Gateway API ────────────────────────────────────────────────────────
	clusters, clustersErr := apigateway.BuildClusterRegistry(ctx, kat, kube, kfg.Cluster().Namespace())
	if clustersErr != nil {
		logger.Fatal().Err(clustersErr).Msg("gateway cluster registry setup failed")
	}

	api, apiErr := apigateway.NewAPIServer(ctx, kat, kube, clusters, kfg.Cluster().Namespace())
	if apiErr != nil {
		logger.Fatal().Err(apiErr).Msg("gateway API setup failed")
	}

	if api != nil {
		api.Register(hs)
	}

	// ── 5. Start ──────────────────────────────────────────────────────────────
	components := []domain.Component{hs, kube}

	o := process.New(
		kfg.RunningInstance(),
		kfg.Catalog().ShutdownGracePeriod(),
		kfg.Inrun().LogLevel(),
	)
	o.Register(components)

	go func() {
		if err := o.Start(ctx); err != nil {
			logger.Fatal().AnErr("gateway startup error", err)
			utils.Exit(err)
		}
	}()

	logger.Warn().Msg("local mode — admission and conversion webhooks are disabled (TLS not available)")
	logger.Info().Msg("gateway started — serving API on HTTP")

	o.Wait()
}
