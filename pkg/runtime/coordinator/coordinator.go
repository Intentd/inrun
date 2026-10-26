package coordinator

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
	"github.com/intentd/inrun/pkg/runtime/queue"
)

// DependencyCoordinator extends the base Controller with dependency‑aware startup.
// It ensures CRDs start in topological order and shut down in reverse order.
type DependencyCoordinator struct {
	*Controller

	depGraph       *blueprint.DependencyGraph
	defaultWorkers int
	startedAt      time.Time
	queueReg       *queue.QueueRegistry
	drainTimeout   time.Duration

	// Inrun and blueprint health
	anyOnline   atomic.Bool
	allOnline   atomic.Bool
	inrunHealth *vitals.RuntimeHealth

	// startedCh[gvk] is closed when a CRD has fully started its workers.
	startedCh map[string]chan struct{}

	// healthyCh[gvk] is closed after the CRD handles first reconciliation.
	healthyCh map[string]chan struct{}

	// missingChildGVKs tracks GVKs declared in onReconcile.custom / onCreate.custom
	// blocks that are not yet available as CRDs in the cluster.
	missingChildGVKs map[string]schema.GroupVersionKind
}

// NewDependencyCoordinator constructs a dependency‑aware coordinator.
// It embeds the base Controller and handles dependencies in the correct order.
func NewDependencyCoordinator(
	kube *kubeclient.Kubeclient,
	factory *informer.Factory,
	observer *observe.Observer,
	blueprint *ResourceBlueprint,
	bp *blueprint.Blueprint,
	events *event.Event,
	hs domain.Health,
	queueRegistry *queue.QueueRegistry,
	defaultWorkqueue *queue.Workqueue,
	crdHealthMap map[string]*vitals.CRDHealth,
	inrunHealth *vitals.RuntimeHealth,
	defaultWorkers int,
	depGraph *blueprint.DependencyGraph,
	drainTimeout time.Duration,
) *DependencyCoordinator {

	coord := &DependencyCoordinator{
		Controller: NewController(
			kube, factory, observer, blueprint, bp,
			events, hs, crdHealthMap, inrunHealth,
			queueRegistry, defaultWorkqueue, defaultWorkers,
		),
		inrunHealth:    inrunHealth,
		depGraph:       depGraph,
		defaultWorkers: defaultWorkers,
		queueReg:       queueRegistry,
		drainTimeout:   drainTimeout,
		startedCh:      make(map[string]chan struct{}),
		healthyCh:      make(map[string]chan struct{}),
	}

	coord.anyOnline.Store(false)
	return coord
}

// Coordinate starts CRDs in dependency order and blocks until leadership is lost.
// When leadership ends, it shuts down CRDs in reverse dependency order.
//
// The startup loop is non‑blocking: if a CRD's dependencies are not yet
// satisfied (e.g., waiting for "healthy"), the CRD is skipped. The background
// retry loop will activate it later when dependencies become ready.
func (dc *DependencyCoordinator) Coordinate(ctx context.Context) {
	logger.Info().Str("component", dc.Name()).Msg("starting")
	dc.startedAt = time.Now()

	// Mark as ready immediately - the coordinator can serve requests
	dc.inrunHealth.SetInrunReady()
	dc.inrunHealth.SetIsLeader(true)

	// Track allOnline
	dc.allOnline.Store(false)
	var totalCRDs, onlineCRDs int

	// Startup order
	startupOrder := dc.depGraph.StartupOrder()
	logger.Info().Str("order", strings.Join(startupOrder, " → ")).Msg("startup order")

	totalCRDs = len(startupOrder)

	// Build name → GVK mapping
	nameToGVK := make(map[string]string)
	for _, name := range startupOrder {
		node := dc.depGraph.GetNode(name)
		if node == nil {
			continue
		}
		nameToGVK[name] = node.CRD.GroupVersionKind.String()
	}

	// Create started + healthy channels for all CRDs
	for _, name := range startupOrder {
		node := dc.depGraph.GetNode(name)
		if node == nil {
			continue
		}
		gvk := node.CRD.GroupVersionKind.String()
		dc.startedCh[gvk] = make(chan struct{})
		dc.healthyCh[gvk] = make(chan struct{})
	}

	// Collect custom child CRDs and detect which are missing at startup.
	// These are GVKs declared in onReconcile.custom / onCreate.custom blocks across all CRDs.
	dc.missingChildGVKs = collectCustomChildGVKs(dc.blueprint)
	for gvkStr, gvk := range dc.missingChildGVKs {
		gvkCopy := gvk
		ok, _ := dc.crdExists(&gvkCopy)
		if ok {
			delete(dc.missingChildGVKs, gvkStr)
		} else {
			logger.Warn().Str("gvk", gvkStr).Msg("custom child CRD not available at startup — will retry")
		}
	}

	// START RETRY LOOP ONCE, BEFORE ANY BLOCKING
	go dc.retryMissingCRDs(ctx)

	// Start dependency health checker (runs until ctx is cancelled)
	go dc.dependencyHealthChecker(ctx)

	// Process CRDs in dependency order — but do NOT block on unsatisfied conditions.
	// Any CRD that cannot start immediately will be picked up by the retry loop.
	for _, name := range startupOrder {
		node := dc.depGraph.GetNode(name)
		if node == nil {
			continue
		}
		crd := node.CRD
		gvk := crd.GroupVersionKind.String()

		// Check if dependencies are satisfied RIGHT NOW
		if !dc.dependenciesReady(crd, nameToGVK) {
			logger.Info().Str("crd", name).Msg("dependencies not ready — deferring activation")
			continue // do NOT block; let retry loop handle it
		}

		// Check if CRD exists in cluster
		if dc.informerFactory.IsMissing(gvk) {
			logger.Debug().Str("crd", name).Str("gvk", gvk).Msg("CRD missing — workers not started, waiting for retry")
			// DO NOT close startedCh or healthyCh — dependents must block
			continue
		}

		// CRD exists — start workers
		workers := dc.blueprint.GetWorkers(gvk, dc.defaultWorkers)
		logger.Info().Str("gvk", gvk).Int("workers", workers).Msg("starting workers")
		dc.startCRDWorkers(ctx, gvk, workers)

		// Update health
		if h, ok := dc.crdHealthMap[gvk]; ok {
			h.SetQueueReg(dc.queueReg)
		}

		// Signal dependents: STARTED ONLY
		close(dc.startedCh[gvk])
		logger.Info().Str("crd", name).Str("gvk", gvk).Int("workers", workers).Msg("workers started")

		// DO NOT close healthyCh here.
		// healthyCh will be closed by the health checker when the CRD becomes healthy.

		dc.anyOnline.Store(true)
		onlineCRDs++
	}

	// Mark controller started
	dc.startedKtrl.Store(true)
	if dc.anyOnline.Load() {
		logger.Info().Str("component", dc.Name()).Int("crds_online", onlineCRDs).Msg("started")
	} else {
		logger.Warn().Str("component", dc.Name()).Msg("started — all CRDs missing, waiting for retry loop")
	}

	// Compute final blueprint health
	if onlineCRDs == totalCRDs {
		dc.allOnline.Store(true)
		dc.inrunHealth.SetAllOnline()
		dc.inrunHealth.SetBlueprintReady()
	} else {
		dc.allOnline.Store(false)
		dc.inrunHealth.SetAllNotOnline()
		dc.inrunHealth.SetBlueprintDegraded()
	}

	// Block until leadership lost
	<-ctx.Done()
	logger.Info().Msg("leadership lost — beginning dependency-aware shutdown")
	dc.hs.Unhealthy()
	dc.inrunHealth.SetIsLeader(false)
	dc.inrunHealth.SetInrunDegraded()

	// Shut down CRDs in reverse dependency order
	shutdownOrder := dc.depGraph.ShutdownOrder()
	logger.Info().Str("order", strings.Join(shutdownOrder, " → ")).Msg("shutdown order")
	for _, name := range shutdownOrder {
		logger.Info().Str("crd", name).Msg("shutting down CRD")
		gvk := dc.depGraph.GetNode(name).CRD.GroupVersionKind.String()
		dc.stopCRDWorkers(ctx, gvk)
	}

	logger.Info().Str("component", dc.Name()).Msg("drained and stopped")
}

// dependenciesReady returns true if all declared dependencies are currently
// satisfied (i.e., the required channel is already closed).
// This check is non‑blocking.
func (dc *DependencyCoordinator) dependenciesReady(crd types.CRDEntry, nameToGVK map[string]string) bool {
	for depName, depCond := range crd.DependsOn {
		depGVK, ok := nameToGVK[depName]
		if !ok {
			logger.Error().Str("crd", crd.Name).Str("dependency", depName).Msg("dependency GVK not found")
			return false
		}
		switch strings.ToLower(depCond.Condition) {
		case string(types.DependencyConditionHealthy):
			select {
			case <-dc.healthyCh[depGVK]:
				// channel closed → dependency healthy
			default:
				return false
			}
		default: // started
			select {
			case <-dc.startedCh[depGVK]:
				// channel closed → dependency started
			default:
				return false
			}
		}
	}
	return true
}

// Name returns the name of the dependency coordinator
func (dc *DependencyCoordinator) Name() string {
	return "inrun dependency coordinator"
}

// NameToCRD returns the CRD for a given name
func (dc *DependencyCoordinator) NameToCRD(name string) types.CRDEntry {
	return dc.depGraph.GetNode(name).CRD
}

// NameToGVK returns the GVK for a given name
func (dc *DependencyCoordinator) NameToGVK(name string) schema.GroupVersionKind {
	return dc.depGraph.GetNode(name).CRD.GroupVersionKind
}

// GVKToCRD returns the CRD entry for a given gvk
func (dc *DependencyCoordinator) GVKToCRD(gvk schema.GroupVersionKind) types.CRDEntry {
	entry, ok := dc.blueprint.Get(gvk.String())
	if !ok {
		return types.CRDEntry{}
	}
	return entry.CRD
}

// NameToGVKMap returns a map of names to gvk string
func (dc *DependencyCoordinator) NameToGVKMap() map[string]string {
	nameToGVK := make(map[string]string)
	for _, name := range dc.depGraph.StartupOrder() {
		node := dc.depGraph.GetNode(name)
		if node != nil {
			nameToGVK[name] = node.CRD.GroupVersionKind.String()
		}
	}
	return nameToGVK
}
