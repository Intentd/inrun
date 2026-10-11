// pkg/runtime/informer/observe/observer.go
package observe

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"github.com/intentd/inrun/pkg/types"
)

// Dependencies are the runtime services required by secondary observers.
//
// Supplied at construction time by the runtime constructor when New() is called.
type Dependencies struct {
	Catalog       domain.Catalog
	Kube          *kubeclient.Kubeclient
	Informer      *informer.Factory
	QueueRegistry *queue.QueueRegistry
}

// Observer creates and manages secondary informers for a CRD.
//
// Observers are intentionally subordinate to the primary CRD informer:
//
//	secondary occurrence
//	    ↓
//	resolve primary key
//	    ↓
//	informer.AllowAndEnqueueKey
//	    ↓
//	primary CR queue
//
// The observer never calls a reconciler directly.
type Observer struct {
	deps Dependencies
}

// New creates an Observer.
func New(deps Dependencies) *Observer {
	return &Observer{deps: deps}
}

// Observe starts all secondary observers declared by the CRD.
//
// This currently covers:
//   - operatorBox.observe.watch
//   - operatorBox.observe.events
//
// Observers are started immediately against ctx.
func (o *Observer) Observe(ctx context.Context, crd types.CRDEntry) {
	o.observeWatches(ctx, crd)
	o.observeEvents(ctx, crd)
}

func (o *Observer) queueFor(crd types.CRDEntry) (*queue.Workqueue, bool) {
	if o.deps.QueueRegistry == nil {
		return nil, false
	}

	return o.deps.QueueRegistry.For(crd.GVKString())
}
