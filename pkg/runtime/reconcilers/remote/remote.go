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

// New returns the reconciler for the declared remote transport.
// Callers import only this package — transport selection is internal.
func New(
	decl *types.RemoteReconcilerDeclaration,
	gvk schema.GroupVersionKind,
	kube kubeclient.Interface,
	ev event.Recorder,
	managedResources []domain.ManagedResource,
	ownNamespace string,
) (domain.Reconciler, error) {
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
