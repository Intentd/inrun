package app

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
)

// newCoordinator builds the observers for secondary resources
// (operatorBox.observe.watch/events) and the dependency coordinator.
//
// The coordinator starts CRD workers in topological order defined by the
// dependency graph. For each CRD, it waits until all declared dependsOn CRDs
// meet their condition (started | healthy) before calling factory() and
// starting workers.
//
// Worker lifecycle:
//
//	Start()  → wait for informer sync → call factory() per worker → run loop
//	Shutdown → drain queue → stop workers → remove from active set
func newCoordinator(
	cfg *config.Config,
	bp *blueprint.Blueprint,
	c core,
	infFactory *informer.Factory,
	registry *coordinator.ResourceBlueprint,
	crdHealthMap map[string]*vitals.CRDHealth,
	inrunHealth *vitals.RuntimeHealth,
) *coordinator.DependencyCoordinator {
	obs := observe.New(observe.Dependencies{
		Kube:          c.kube,
		Informer:      infFactory,
		QueueRegistry: c.queueRegistry,
		Blueprint:     bp,
	})

	return coordinator.NewDependencyCoordinator(
		c.kube,
		infFactory,
		obs,
		registry,
		bp,
		c.ev,
		c.hs,
		c.queueRegistry,
		c.defaultWq,
		crdHealthMap,
		inrunHealth,
		cfg.Blueprint().DefaultWorkers(),
		blueprint.NewDependencyGraph(bp),
		cfg.Blueprint().ShutdownTimeout(),
	)
}
