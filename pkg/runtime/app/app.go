// Package app assembles the Inrun runtime: the Blueprint, Kubernetes clients and
// informers, every CRD's queue, informer and reconciler, the HTTP routes, the
// coordinator, and the process that starts and stops them. New wires all of it
// and starts nothing; the caller starts the process. Being importable, it lets
// tests and tools run a real runtime in-process.
package app

import (
	"context"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/process"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer/crds"
)

// Runtime is the assembled runtime. Start it with Process.Start; Coordinator
// begins reconciling once this replica leads.
type Runtime struct {
	Blueprint   *blueprint.Blueprint
	Event       *event.Event
	Kube        *kubeclient.Kubeclient
	Coordinator *coordinator.DependencyCoordinator
	Process     *process.Manager
}

// New wires the entire Inrun runtime.
//
// Nothing is started here. Every component is constructed and threaded together
// as closures and pointers. Process.Start() calls component.Start() in
// registration order, and component.Stop() in reverse order on shutdown.
// Each step below is its own function; read top to bottom, this is the
// runtime's dependency graph and startup order.
func New(cfg *config.Config, m *merger.Merger, ctx context.Context) (*Runtime, error) {
	bp := pipeline.NewBlueprint(cfg, m, config.Runtime())
	c, err := newCore(ctx, cfg, bp)
	if err != nil {
		return nil, err
	}

	infFactory := crds.NewFactory(cfg, bp, c.kube, c.scheme, c.defaultWq, c.queueRegistry)

	registry, crdHealthMap, crdErr := wireCRDs(ctx, cfg, bp, c, infFactory)
	if crdErr != nil {
		return nil, crdErr
	}

	inrunHealth := vitals.NewRuntimeHealth()
	registerRoutes(c.hs, cfg, m, bp, c.kube, registry, crdHealthMap, inrunHealth)
	coord := newCoordinator(cfg, bp, c, infFactory, registry, crdHealthMap, inrunHealth)
	o := process.New(cfg, components(c, infFactory, coord))

	return &Runtime{
		Blueprint:   bp,
		Event:       c.ev,
		Kube:        c.kube,
		Coordinator: coord,
		Process:     o,
	}, nil
}
