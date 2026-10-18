package app

import (
	"strings"

	"github.com/intentd/inrun/pkg/runtimeapi"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
)

// registerRoutes registers the runtime's HTTP routes. All routes are
// registered before hs.Start() — the mux is shared.
//
// Per-CRD routes:
//
//	/catalog/{crd}/health                 200 healthy, 503 degraded
//	/catalog/{crd}                        CRD config and live reconcile stats
//	/catalog/{crd}/raw                    the user's config
//	/catalog/{crd}/enriched               the runtime config
//	/catalog/{crd}/cr                     all CR instances (informer cache)
//	/catalog/{crd}/cr/{ns}/{name}         CR detail and children
//	/catalog/{crd}/cr/{ns}/{name}/events  recent events
//
// Aggregate:
//
//	/catalog/raw                          the user's Catalog
//	/catalog/enriched                     the runtime Catalog
//	/catalog                              all CRDs, dependency graph, health summary
func registerRoutes(
	hs *health.HealthServer,
	cfg *config.Config,
	m *merger.Merger,
	cat *catalog.Catalog,
	kube *kubeclient.Kubeclient,
	registry *coordinator.ResourceCatalog,
	crdHealthMap map[string]*vitals.CRDHealth,
	inrunHealth *vitals.RuntimeHealth,
) {
	for _, crd := range cat.Enabled() {
		gvk := crd.GVKString()
		crdHealth := crdHealthMap[gvk]
		crdName := strings.ToLower(crd.Name)

		entry, _ := registry.Get(gvk)
		inf := entry.Informer

		if !crd.IsEnabledAllEndpoints() {
			continue
		}

		if crd.IsHealthEnabled() {
			hs.Register(
				runtimeapi.HealthPath(crdName),
				vitals.BuildCRDHealthHandler(crd, cfg, inf, crdHealth, inrunHealth),
			)
		}

		if crd.IsInfoEnabled() {
			hs.Register(
				runtimeapi.CRDPath(crdName),
				vitals.BuildCRDInfoHandler(crd, cat.RBACRulesForCRD(crd), cfg, inf, crdHealth, inrunHealth),
			)
			hs.Register(
				runtimeapi.CRListPath(crdName),
				vitals.BuildCRListHandler(crd, inf, inrunHealth),
			)
			hs.Register(
				runtimeapi.CRListPath(crdName)+"/",
				vitals.BuildCRDetailAndEventsHandler(crd, inf, kube, crd.Box(), inrunHealth),
			)
		}

		// Register raw and enriched CRD definition endpoint
		hs.Register(
			runtimeapi.RawPath(crdName),
			vitals.BuildCRDRawHandler(m, crd.Name),
		)
		hs.Register(
			runtimeapi.EnrichedPath(crdName),
			vitals.BuildCRDEnrichedHandler(cat, crd.Name),
		)

		logger.Debug().
			Str("health", runtimeapi.HealthPath(crdName)).
			Str("info", runtimeapi.CRDPath(crdName)).
			Str("raw", runtimeapi.RawPath(crdName)).
			Str("enriched", runtimeapi.EnrichedPath(crdName)).
			Msg("registered CRD routes")
	}

	hs.Register(runtimeapi.CatalogPath+"/raw", vitals.BuildRawCatalogHandler(m))
	hs.Register(runtimeapi.CatalogPath+"/enriched", vitals.BuildEnrichedCatalogHandler(cat))
	hs.Register(runtimeapi.CatalogPath, vitals.BuildCatalogHandler(cat, cfg, registry, crdHealthMap, inrunHealth))
}
