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

	"github.com/intentd/inrun/pkg/catalog"
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

	depGraph       *catalog.DependencyGraph
	defaultWorkers int
	startedAt      time.Time
	queueReg       *queue.QueueRegistry
	drainTimeout   time.Duration

	// Inrun and catalog health
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
	catalog *ResourceCatalog,
	kat *catalog.Catalog,
	events *event.Event,
	hs domain.Health,
	queueRegistry *queue.QueueRegistry,
	defaultWorkqueue *queue.Workqueue,
	crdHealthMap map[string]*vitals.CRDHealth,
	inrunHealth *vitals.RuntimeHealth,
	defaultWorkers int,
	depGraph *catalog.DependencyGraph,
	drainTimeout time.Duration,
) *DependencyCoordinator {

	kord := &DependencyCoordinator{
		Controller: NewController(
			kube, factory, observer, catalog, kat,
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

	kord.anyOnline.Store(false)
	return kord
}

// Coordinate starts CRDs in dependency order and blocks until leadership is lost.
// When leadership ends, it shuts down CRDs in reverse dependency order.
//
// The startup loop is non‑blocking: if a CRD's dependencies are not yet
// satisfied (e.g., waiting for "healthy"), the CRD is skipped. The background
// retry loop will activate it later when dependencies become ready.
func (k *DependencyCoordinator) Coordinate(ctx context.Context) {
	logger.Info().Str("component", k.Name()).Msg("starting")
	k.startedAt = time.Now()

	// Mark as ready immediately - the coordinator can serve requests
	k.inrunHealth.SetInrunReady()
	k.inrunHealth.SetIsLeader(true)

	// Track allOnline
	k.allOnline.Store(false)
	var totalCRDs, onlineCRDs int

	// Startup order
	startupOrder := k.depGraph.StartupOrder()
	logger.Info().Str("order", strings.Join(startupOrder, " → ")).Msg("startup order")

	totalCRDs = len(startupOrder)

	// Build name → GVK mapping
	nameToGVK := make(map[string]string)
	for _, name := range startupOrder {
		node := k.depGraph.GetNode(name)
		if node == nil {
			continue
		}
		nameToGVK[name] = node.CRD.GroupVersionKind.String()
	}

	// Create started + healthy channels for all CRDs
	for _, name := range startupOrder {
		node := k.depGraph.GetNode(name)
		if node == nil {
			continue
		}
		gvk := node.CRD.GroupVersionKind.String()
		k.startedCh[gvk] = make(chan struct{})
		k.healthyCh[gvk] = make(chan struct{})
	}

	// Collect custom child CRDs and detect which are missing at startup.
	// These are GVKs declared in onReconcile.custom / onCreate.custom blocks across all CRDs.
	k.missingChildGVKs = collectCustomChildGVKs(k.catalog)
	for gvkStr, gvk := range k.missingChildGVKs {
		gvkCopy := gvk
		ok, _ := k.crdExists(&gvkCopy)
		if ok {
			delete(k.missingChildGVKs, gvkStr)
		} else {
			logger.Warn().Str("gvk", gvkStr).Msg("custom child CRD not available at startup — will retry")
		}
	}

	// START RETRY LOOP ONCE, BEFORE ANY BLOCKING
	go k.retryMissingCRDs(ctx)

	// Start dependency health checker (runs until ctx is cancelled)
	go k.dependencyHealthChecker(ctx)

	// Process CRDs in dependency order — but do NOT block on unsatisfied conditions.
	// Any CRD that cannot start immediately will be picked up by the retry loop.
	for _, name := range startupOrder {
		node := k.depGraph.GetNode(name)
		if node == nil {
			continue
		}
		crd := node.CRD
		gvk := crd.GroupVersionKind.String()

		// Check if dependencies are satisfied RIGHT NOW
		if !k.dependenciesReady(crd, nameToGVK) {
			logger.Info().Str("crd", name).Msg("dependencies not ready — deferring activation")
			continue // do NOT block; let retry loop handle it
		}

		// Check if CRD exists in cluster
		if k.informerFactory.IsMissing(gvk) {
			logger.Debug().Str("crd", name).Str("gvk", gvk).Msg("CRD missing — workers not started, waiting for retry")
			// DO NOT close startedCh or healthyCh — dependents must block
			continue
		}

		// CRD exists — start workers
		workers := k.catalog.GetWorkers(gvk, k.defaultWorkers)
		logger.Info().Str("gvk", gvk).Int("workers", workers).Msg("starting workers")
		k.startCRDWorkers(ctx, gvk, workers)

		// Update health
		if h, ok := k.crdHealthMap[gvk]; ok {
			h.SetQueueReg(k.queueReg)
		}

		// Signal dependents: STARTED ONLY
		close(k.startedCh[gvk])
		logger.Info().Str("crd", name).Str("gvk", gvk).Int("workers", workers).Msg("workers started")

		// DO NOT close healthyCh here.
		// healthyCh will be closed by the health checker when the CRD becomes healthy.

		k.anyOnline.Store(true)
		onlineCRDs++
	}

	// Mark controller started
	k.startedKtrl.Store(true)
	if k.anyOnline.Load() {
		logger.Info().Str("component", k.Name()).Int("crds_online", onlineCRDs).Msg("started")
	} else {
		logger.Warn().Str("component", k.Name()).Msg("started — all CRDs missing, waiting for retry loop")
	}

	// Compute final catalog health
	if onlineCRDs == totalCRDs {
		k.allOnline.Store(true)
		k.inrunHealth.SetAllOnline()
		k.inrunHealth.SetCatalogReady()
	} else {
		k.allOnline.Store(false)
		k.inrunHealth.SetAllNotOnline()
		k.inrunHealth.SetCatalogDegraded()
	}

	// Block until leadership lost
	<-ctx.Done()
	logger.Info().Msg("leadership lost — beginning dependency-aware shutdown")
	k.hs.Unhealthy()
	k.inrunHealth.SetIsLeader(false)
	k.inrunHealth.SetInrunDegraded()

	// Shut down CRDs in reverse dependency order
	shutdownOrder := k.depGraph.ShutdownOrder()
	logger.Info().Str("order", strings.Join(shutdownOrder, " → ")).Msg("shutdown order")
	for _, name := range shutdownOrder {
		logger.Info().Str("crd", name).Msg("shutting down CRD")
		gvk := k.depGraph.GetNode(name).CRD.GroupVersionKind.String()
		k.stopCRDWorkers(ctx, gvk)
	}

	logger.Info().Str("component", k.Name()).Msg("drained and stopped")
}

// dependenciesReady returns true if all declared dependencies are currently
// satisfied (i.e., the required channel is already closed).
// This check is non‑blocking.
func (k *DependencyCoordinator) dependenciesReady(crd types.CRDEntry, nameToGVK map[string]string) bool {
	for depName, depCond := range crd.DependsOn {
		depGVK, ok := nameToGVK[depName]
		if !ok {
			logger.Error().Str("crd", crd.Name).Str("dependency", depName).Msg("dependency GVK not found")
			return false
		}
		switch strings.ToLower(depCond.Condition) {
		case string(types.DependencyConditionHealthy):
			select {
			case <-k.healthyCh[depGVK]:
				// channel closed → dependency healthy
			default:
				return false
			}
		default: // started
			select {
			case <-k.startedCh[depGVK]:
				// channel closed → dependency started
			default:
				return false
			}
		}
	}
	return true
}

// Name returns the name of the dependency coordinator
func (k *DependencyCoordinator) Name() string {
	return "inrun dependency coordinator"
}

// NameToCRD returns the CRD for a given name
func (k *DependencyCoordinator) NameToCRD(name string) types.CRDEntry {
	return k.depGraph.GetNode(name).CRD
}

// NameToGVK returns the GVK for a given name
func (k *DependencyCoordinator) NameToGVK(name string) schema.GroupVersionKind {
	return k.depGraph.GetNode(name).CRD.GroupVersionKind
}

// GVKToCRD returns the CRD entry for a given gvk
func (k *DependencyCoordinator) GVKToCRD(gvk schema.GroupVersionKind) types.CRDEntry {
	entry, ok := k.catalog.Get(gvk.String())
	if !ok {
		return types.CRDEntry{}
	}
	return entry.CRD
}

// NameToGVKMap returns a map of names to gvk string
func (k *DependencyCoordinator) NameToGVKMap() map[string]string {
	nameToGVK := make(map[string]string)
	for _, name := range k.depGraph.StartupOrder() {
		node := k.depGraph.GetNode(name)
		if node != nil {
			nameToGVK[name] = node.CRD.GroupVersionKind.String()
		}
	}
	return nameToGVK
}
