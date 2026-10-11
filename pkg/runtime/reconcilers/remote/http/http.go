package http

import (
	"context"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const defaultRemoteTimeout = 30 * time.Second

// Reconciler implements domain.Reconciler by POSTing a PreparedRequest
// to a remote HTTP endpoint. The remote service owns the reconcile logic;
// Inrun owns the queue, backoff, informer, health, and metrics.
type Reconciler struct {
	decl             *types.RemoteReconcilerDeclaration
	gvk              schema.GroupVersionKind
	kube             kubeclient.Interface
	event            event.Recorder
	managedResources []domain.ManagedResource
	namespace        string // inrun namespace, used as default when secretRef.namespace is empty
	client           httpDoer
}

// New constructs a Reconciler for the given CRD.
// kube is used for credential resolution and for reading the CR's children.
// ev is used to emit Kubernetes events on reconcile success or failure.
// managedResources lists the child types sent to the remote reconciler.
// ownNamespace is the Inrun namespace (used when secretRef.namespace is empty).
func New(
	decl *types.RemoteReconcilerDeclaration,
	gvk schema.GroupVersionKind,
	kube kubeclient.Interface,
	ev event.Recorder,
	managedResources []domain.ManagedResource,
	ownNamespace string,
) *Reconciler {
	timeout := defaultRemoteTimeout
	if decl.Timeout.Duration > 0 {
		timeout = decl.Timeout.Duration
	}
	return &Reconciler{
		decl:             decl,
		gvk:              gvk,
		kube:             kube,
		event:            ev,
		managedResources: managedResources,
		namespace:        ownNamespace,
		client:           newHTTPClient(timeout),
	}
}

// Reconcile dispatches the request to the remote endpoint and returns its
// Result. The coordinator performs every write the Result asks for, including
// applying the returned resources.
func (r *Reconciler) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	log := logger.FromContext(ctx).With().
		Str("reconciler", "remote").
		Str("endpoint", r.resolveEndpoint(req)).
		Logger()

	log.Info().Str("key", req.Key).Msg("dispatching to remote reconciler")

	raw, err := r.callRemote(ctx, req)
	if err != nil {
		return domain.Result{}, err
	}
	return r.toResult(ctx, req, raw)
}
