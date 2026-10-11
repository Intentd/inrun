package app

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"k8s.io/apimachinery/pkg/runtime"
)

// core holds the components every later step is wired from.
type core struct {
	scheme        *runtime.Scheme
	kube          *kubeclient.Kubeclient
	hs            *health.HealthServer
	ev            *event.Event
	defaultWq     *queue.Workqueue
	queueRegistry *queue.QueueRegistry
}

// newCore builds the scheme and the core components, created here and started
// later by inrun in registration order.
func newCore(ctx context.Context, cfg *config.Config, cat *catalog.Catalog) core {
	// Each CRD type (e.g. *PipelineList) must be registered with the scheme so
	// the REST client knows how to decode API server responses. For dynamic CRDs
	// (unstructured mode), this is a no-op — they use the dynamic client.
	scheme, err := catalog.NewSchemeRegistry(cat)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build scheme registry")
	}

	kube := kubeclient.NewKubeclient(cfg, scheme)

	// kubeclient is started immediately — the informer factory's missing-CRD
	// check needs the REST config during construction, before Process.Start().
	if err := kube.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start kubeclient")
	}

	return core{
		scheme: scheme,
		kube:   kube,
		// HTTP-only (health, readiness, metrics, Catalog API routes). Routes are
		// registered before Start() binds the port.
		hs: health.NewHealthServer(cfg),
		// Surfaces notable state changes to the Kubernetes event stream (visible
		// via kubectl describe). Shared by all controllers that emit events.
		ev: event.NewEvent(kube),
		// Rate-limiting reconcile queue shared by controllers that do not need a
		// dedicated queue. Bounded to prevent runaway reconcile storms.
		defaultWq: queue.NewWorkqueue("default workqueue"),
		// Maps GVK strings to dedicated work queues, so cross-controller
		// enqueues are dispatched correctly.
		queueRegistry: queue.NewQueueRegistry(),
	}
}

// components lists the runtime's components in start order: each starts after
// its dependencies, and they stop in reverse order (automatic).
//
// HealthServer starts first so it can serve /ready during startup. Kubeclient
// is already started (newCore) but is still registered so inrun manages its
// Stop().
func components(c core, infFactory *informer.Factory, coord *coordinator.DependencyCoordinator) []domain.Component {
	return []domain.Component{
		c.hs,            // 1. HTTP server — /ready, /livez, /catalog routes
		c.kube,          // 2. REST clients — already started, managed for Stop()
		c.ev,            // 3. event recorder — depends on kube
		c.queueRegistry, // 4. per-CRD bounded queues
		c.defaultWq,     // 5. default unbounded queue
		infFactory,      // 6. informer factory — starts watchers, closes ready channel
		coord,           // 7. dependency coordinator — starts workers in topo order
	}
}
