package app

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/config"
	apigateway "github.com/intentd/inrun/pkg/gateway/api"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/merger"
)

// buildBase marks this process as the gateway, loads the Blueprint (it says which
// CRDs need webhooks and which are served) and starts the kubeclient.
func buildBase(ctx context.Context, cfg *config.Config, m *merger.Merger) (*blueprint.Blueprint, *kubeclient.Kubeclient, error) {
	bp := pipeline.NewBlueprint(cfg, m, config.Gateway())

	scheme, err := blueprint.NewSchemeRegistry(bp)
	if err != nil {
		return nil, nil, fmt.Errorf("build scheme registry: %w", err)
	}

	kube := kubeclient.New(cfg, scheme)
	if err := kube.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("start kubeclient: %w", err)
	}
	return bp, kube, nil
}

// newAPI builds the Gateway API — POST /api/v1/apply, GET/DELETE
// /api/v1/resources/, GET /api/v1/schema/ — or nil when gateway.api is not
// enabled or no CRD is served. The cluster registry it builds is shared by the
// API server and the intake server.
func newAPI(ctx context.Context, cfg *config.Config, bp *blueprint.Blueprint, kube *kubeclient.Kubeclient) (*apigateway.APIServer, error) {
	clusters, err := apigateway.BuildClusterRegistry(ctx, bp, kube, cfg.Cluster().Namespace())
	if err != nil {
		return nil, fmt.Errorf("gateway cluster registry setup: %w", err)
	}
	api, err := apigateway.NewAPIServer(ctx, bp, kube, clusters, cfg.Cluster().Namespace())
	if err != nil {
		return nil, fmt.Errorf("gateway API setup: %w", err)
	}
	return api, nil
}
