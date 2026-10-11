package app

import (
	"strings"

	"github.com/intentd/inrun/pkg/catalog"
	gwhandlers "github.com/intentd/inrun/pkg/gateway/handlers"
	"github.com/intentd/inrun/pkg/gateway/webhook"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/logger"
)

// registerCatalogRoutes serves the gateway's own stats surface. The console
// discovers it through the "gatewayEndpoint" field of the runtime's /catalog
// response and merges per-CRD stats by GVR key.
func registerCatalogRoutes(hs *health.HealthServer, cat *catalog.Catalog, ws *webhook.WebhookServer) {
	hs.Register("/catalog", gwhandlers.BuildGatewayCatalogHandler(cat, ws))

	for _, crd := range cat.Enabled() {
		crdName := strings.ToLower(crd.Name)
		gvr := crd.GVR()
		gvrKey := webhook.GVRKey(gvr.Group, gvr.Version, gvr.Resource)
		hs.Register(
			"/catalog/"+crdName,
			gwhandlers.BuildGatewayCRDHandler(crd.Name, crd.GVKString(), gvr.String(), gvrKey, ws, cat),
		)
	}
	logger.Debug().Msg("gateway /catalog routes registered")
}
