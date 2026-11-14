package app

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/runtime/coordinator"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/queue"
	kubeutil "github.com/intentd/tools/kube"
	"github.com/intentd/tools/kubeclient"
	"k8s.io/apimachinery/pkg/runtime"
)

// Events is wired as one of the runtime's components.
var _ domain.Component = (*kubeutil.Events)(nil)

// core holds the components every later step is wired from.
type core struct {
	scheme        *runtime.Scheme
	kube          *kubeclient.Kubeclient
	hs            *health.HealthServer
	ev            *kubeutil.Events
	defaultWq     *queue.Workqueue
	queueRegistry *queue.QueueRegistry
}

// newCore builds the scheme and the core components, created here and started
// later by inrun in registration order.
func newCore(ctx context.Context, cfg *config.Config, bp *blueprint.Blueprint) (core, error) {
	// Each CRD type (e.g. *PipelineList) must be registered with the scheme so
	// the REST client knows how to decode API server responses. For dynamic CRDs
	// (unstructured mode), this is a no-op — they use the dynamic client.
	scheme, err := blueprint.NewSchemeRegistry(bp)
	if err != nil {
		return core{}, fmt.Errorf("failed to build scheme registry: %w", err)
	}

	kube := kubeclient.New(kubeclient.Options{KubeconfigPath: cfg.Cluster().KubeconfigPath(), MasterURL: cfg.Cluster().MasterURL()}, scheme)

	// kubeclient is started immediately — the informer factory's missing-CRD
	// check needs the REST config during construction, before Process.Start().
	if err := kube.Start(ctx); err != nil {
		return core{}, fmt.Errorf("failed to start kubeclient: %w", err)
	}

	// Surfaces notable state changes to the Kubernetes event stream (visible
	// via kubectl describe). Shared by all controllers that emit events.
	ev, err := kubeutil.NewEvents(kube.Clientset(), kube.Scheme(), "inrun runtime")
	if err != nil {
		return core{}, err
	}

	return core{
		scheme: scheme,
		kube:   kube,
		// HTTP-only (health, readiness, metrics, Blueprint API routes). Routes are
		// registered before Start() binds the port.
		hs: health.NewHealthServer(cfg),
		ev: ev,
		// Rate-limiting reconcile queue shared by controllers that do not need a
		// dedicated queue. Bounded to prevent runaway reconcile storms.
		defaultWq: queue.NewWorkqueue("default workqueue"),
		// Maps GVK strings to dedicated work queues, so cross-controller
		// enqueues are dispatched correctly.
		queueRegistry: queue.NewQueueRegistry(),
	}, nil
}

// components lists the runtime's components in start order: each starts after
// its dependencies, and they stop in reverse order (automatic).
//
// HealthServer starts first so it can serve /ready during startup. Kubeclient
// is already started (newCore) but is still registered so inrun manages its
// Stop().
func components(c core, infFactory *informer.Factory, coord *coordinator.DependencyCoordinator) []domain.Component {
	return []domain.Component{
		c.hs,            // 1. HTTP server — /ready, /livez, /blueprint routes
		c.kube,          // 2. REST clients — already started, managed for Stop()
		c.ev,            // 3. event recorder — depends on kube
		c.queueRegistry, // 4. per-CRD bounded queues
		c.defaultWq,     // 5. default unbounded queue
		infFactory,      // 6. informer factory — starts watchers, closes ready channel
		coord,           // 7. dependency coordinator — starts workers in topo order
	}
}
