package app

import (
	"github.com/intentd/inrun/pkg/util/kube"
	"strings"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/gateway/housekeeper"
	"github.com/intentd/inrun/pkg/gateway/stats"
	"github.com/intentd/inrun/pkg/gateway/webhook"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/logger"
)

// registerBlueprintRoutes serves the gateway's own stats surface. The console
// discovers it through the "gatewayEndpoint" field of the runtime's /blueprint
// response and merges per-CRD stats by GVR key.
func registerBlueprintRoutes(hs *health.HealthServer, bp *blueprint.Blueprint, ws *webhook.Server, hk *housekeeper.Housekeeper) {
	hs.Register("/blueprint", stats.Handler(bp, ws, hk))

	for _, crd := range bp.Enabled() {
		crdName := strings.ToLower(crd.Name)
		gvr := crd.GVR()
		gvrKey := kube.GVRKey(gvr.Group, gvr.Version, gvr.Resource)
		hs.Register(
			"/blueprint/"+crdName,
			stats.CRDHandler(crd.Name, crd.GVKString(), gvr.String(), gvrKey, ws, bp),
		)
	}
	logger.Debug().Msg("gateway /blueprint routes registered")
}
