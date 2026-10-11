package coordinator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"errors"

	"github.com/intentd/inrun/domain"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/types"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/informer/observe"
	"github.com/intentd/inrun/pkg/runtime/queue"
)

var _ domain.Component = (*Controller)(nil)

// Every map has the same key GVK
type Controller struct {
	kube             *kubeclient.Kubeclient
	informerFactory  *informer.Factory
	observer         *observe.Observer
	event            *event.Event
	catalog          *ResourceCatalog
	cat              *catalog.Catalog
	queueRegistry    *queue.QueueRegistry
	defaultWorkqueue *queue.Workqueue
	failureThreshold map[string]int

	hs           domain.Health
	crdHealthMap map[string]*vitals.CRDHealth
	inrunHealth  *vitals.RuntimeHealth

	defaultWorkers int
	startedKtrl    atomic.Bool
	started        map[string]bool
	deactivated    map[string]bool
	cancelFuncs    map[string]context.CancelFunc
	wgs            map[string]*sync.WaitGroup
	mu             sync.RWMutex
	reconcilers    map[string]domain.Reconciler
	crds           []types.CRDEntry

	// runtimeMap holds per-CRD concurrency and autoscale state (semaphore,
	// AutoMetrics, Autoscaler, resync interval). Populated by startCRDWorkers.
	runtimeMap map[string]*perCRDRuntime

	// Error rate
	total  map[string]int
	failed map[string]int
}

func NewController(
	kube *kubeclient.Kubeclient,
	informerFactory *informer.Factory,
	observer *observe.Observer,
	catalog *ResourceCatalog,
	cat *catalog.Catalog,
	event *event.Event,
	hs domain.Health,
	crdHealthMap map[string]*vitals.CRDHealth,
	inrunHealth *vitals.RuntimeHealth,
	queueRegistry *queue.QueueRegistry,
	defaultWorkqueue *queue.Workqueue,
	defaultWorkers int,
) *Controller {
	k := &Controller{
		kube:             kube,
		informerFactory:  informerFactory,
		observer:         observer,
		catalog:          catalog,
		cat:              cat,
		event:            event,
		hs:               hs,
		defaultWorkqueue: defaultWorkqueue,
		queueRegistry:    queueRegistry,
		defaultWorkers:   defaultWorkers,
		crdHealthMap:     crdHealthMap,
		started:          make(map[string]bool),
		deactivated:      make(map[string]bool),
		cancelFuncs:      make(map[string]context.CancelFunc),
		total:            make(map[string]int),
		failed:           make(map[string]int),
		wgs:              make(map[string]*sync.WaitGroup),
		reconcilers:      make(map[string]domain.Reconciler),
		failureThreshold: make(map[string]int),
		runtimeMap:       make(map[string]*perCRDRuntime),
	}

	// Load registry entries
	for gvk, entry := range catalog.Entries() {
		k.crds = append(k.crds, entry.CRD)
		k.failureThreshold[gvk] = entry.CRD.SetFailureThreshold(0)
	}

	return k
}

func (k *Controller) Start(ctx context.Context) error {
	// CRD checks now carried out in informer
	// Controller just accepts that crds have been checked, listens to know if any is missing
	// Then marks as degraded
	for _, crd := range k.crds {
		gvk := crd.GroupVersionKind.String()

		if !k.informerFactory.IsMissing(gvk) {
			continue
		}
		logger.Warn().Str("gvk", gvk).Msg("CRD missing — marking as degraded")
		k.crdHealthMap[gvk].RecordStartupFailure(errors.New("CRD not found"), crd.SetFailureThreshold(0))
	}

	// All CRDs confirmed (filtered by informer) — now sync caches
	logger.Debug().Msg("waiting for all informer caches to sync...")
	if !k.informerFactory.WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to sync one or more informer caches")
	}
	logger.Info().Msg("all informer caches synced")

	// Build reconcilers now — kube, ev, and REST clients are all live
	logger.Debug().Msg("building reconcilers...")
	for gvk, entry := range k.catalog.Entries() {
		rec := entry.ReconcilerFactory() // ← safe here, manager has started everything
		k.mu.Lock()
		k.reconcilers[gvk] = rec
		k.mu.Unlock()
		logger.Debug().Str("gvk", gvk).Msg("reconciler built")
	}

	return nil
}

// MissingCRDs returns a map of missing crds keyed by gvk
func (k *Controller) MissingCRDs() map[string]*informer.InformerEntry {
	return k.informerFactory.Missing()
}

// Set the controller ready
func (k *Controller) SetReady(h domain.Health) {
	h.SetReady()
}

// Set the controller to degraded
func (k *Controller) Degraded(h domain.Health) {
	h.Degraded()
}

// Healthy mark on startup
func (k *Controller) Started() bool { return k.startedKtrl.Load() }

// Shutdown gracefully stops inrun
func (k *Controller) Shutdown(ctx context.Context) {}

// Controller name
func (k *Controller) Name() string {
	return "inrun controller"
}

// Handle failure writes for concurrency
func (k *Controller) failedReconcile(gvk string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.failed[gvk]++
}

// Handle success writes for concurrency
func (k *Controller) successReconcile(gvk string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.total[gvk]++
}
