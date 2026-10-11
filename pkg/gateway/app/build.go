package app

import (
	"context"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/config"
	apigateway "github.com/intentd/inrun/pkg/gateway/api"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
)

// buildBase marks this process as the gateway, loads the Catalog (it says which
// CRDs need webhooks and which are served) and starts the kubeclient.
func buildBase(ctx context.Context, cfg *config.Config, m *merger.Merger) (*catalog.Catalog, *kubeclient.Kubeclient) {
	cat := pipeline.NewCatalog(cfg, m, config.Gateway())

	scheme, err := catalog.NewSchemeRegistry(cat)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build scheme registry")
	}

	kube := kubeclient.NewKubeclient(cfg, scheme)
	if err := kube.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start kubeclient")
	}
	return cat, kube
}

// newAPI builds the Gateway API — POST /api/v1/apply, GET/DELETE
// /api/v1/resources/, GET /api/v1/schema/ — or nil when gateway.api is not
// enabled or no CRD is served. The cluster registry it builds is shared by the
// API server and the intake server.
func newAPI(ctx context.Context, cfg *config.Config, cat *catalog.Catalog, kube *kubeclient.Kubeclient) *apigateway.APIServer {
	clusters, err := apigateway.BuildClusterRegistry(ctx, cat, kube, cfg.Cluster().Namespace())
	if err != nil {
		logger.Fatal().Err(err).Msg("gateway cluster registry setup failed")
	}
	api, err := apigateway.NewAPIServer(ctx, cat, kube, clusters, cfg.Cluster().Namespace())
	if err != nil {
		logger.Fatal().Err(err).Msg("gateway API setup failed")
	}
	return api
}
