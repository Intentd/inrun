package app

import (
	"context"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/informer/crds"
	"github.com/intentd/inrun/pkg/runtime/reconcilers"
)

// wireCRDs registers every enabled CRD with the coordinator registry: its
// queue, its informer and its reconciler factory. It also builds one CRDHealth
// per CRD, shared between the coordinator (which updates it on each reconcile)
// and the HTTP health routes (which read it on each request).
func wireCRDs(ctx context.Context, cfg *config.Config, cat *catalog.Catalog, c core, infFactory *informer.Factory) (*coordinator.ResourceCatalog, map[string]*vitals.CRDHealth) {
	// The registry maps GVK → (CRDEntry, SharedIndexInformer, ReconcilerFactory).
	// It also implements reconciler.CatalogRegistry via GetInformerByName,
	// enabling cross-CRD observation with zero API server calls.
	registry := coordinator.NewCoordinatorRegistry()

	crdHealthMap := make(map[string]*vitals.CRDHealth)
	for _, crd := range cat.Enabled() {
		crdHealthMap[crd.GVKString()] = vitals.NewCRDHealth(crd.Name)
	}

	logger.Debug().Msg("wiring CRDs into coordinator registry...")

	for _, crd := range cat.Enabled() {
		gvk := crd.GVKString()
		inf := crds.Informer(ctx, crd, c.kube, infFactory, c.queueRegistry)

		// The factory is called by startCRDWorkers after informers are synced;
		// each call returns a fresh reconciler for one worker goroutine.
		factory, err := reconcilers.Factory(crd, reconcilers.Deps{
			Kube:         c.kube,
			Informer:     inf,
			Event:        c.ev,
			StoreFor:     infFactory.StoreFor,
			IndexerFor:   infFactory.IndexerFor,
			Catalog:      cat,
			OwnNamespace: cfg.Cluster().Namespace(),
		})
		if err != nil {
			logger.Fatal().Err(err).Str("gvk", gvk).Msg("failed to build the reconciler")
		}

		// Register informs the DependencyCoordinator which informer and factory
		// belong to this CRD. Workers are not started yet — that happens in Start().
		registry.Register(gvk, crd, inf, factory)
		logger.Debug().Str("gvk", gvk).Msg("CRD registered")
	}
	return registry, crdHealthMap
}
