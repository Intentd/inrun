package remote

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/runtime/reconcilers/remote/http"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Transport is what each remote protocol implements: Reconcile for a remote
// on its own, and Hooks for a mapped remote with templates, where the call is
// the generic reconciler's hook (see http.Reconciler.Hooks).
type Transport interface {
	domain.Reconciler
	Hooks() domain.ObjectHooks
}

// New returns the transport for the declared protocol (HTTP when unset).
// Callers import only this package — transport selection is internal.
func New(
	decl *types.RemoteReconcilerDeclaration,
	gvk schema.GroupVersionKind,
	kube kubeclient.Interface,
	ev event.Recorder,
	managedResources []domain.ManagedResource,
	ownNamespace string,
) (Transport, error) {
	t := decl.Protocol
	if t == "" {
		t = types.RemoteReconcileProtocolHTTP
	}
	switch t {
	case types.RemoteReconcileProtocolHTTP:
		return http.New(decl, gvk, kube, ev, managedResources, ownNamespace), nil
	default:
		return nil, fmt.Errorf("remote reconciler: unsupported transport %q", t)
	}
}
