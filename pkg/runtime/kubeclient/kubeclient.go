// Package kubeclient is the Kubernetes client Inrun's reconcilers and hooks
// receive. The client itself is github.com/intentd/tools/kubeclient; this
// package names it for Inrun's users and adds ApplyOptions, how a
// reconciler writes a child resource with kubewright.
package kubeclient

import (
	"context"

	"github.com/intentd/inrun/pkg/config"
	kwtypes "github.com/intentd/kubewright/core/types"
	"github.com/intentd/tools/kubeclient"
)

// Interface is the client a reconciler or hook receives.
type Interface = kubeclient.Interface

// Kubeclient is the client implementation typed reconciler constructors receive.
type Kubeclient = kubeclient.Kubeclient

// FromContext returns the client stored in ctx, if any.
func FromContext(ctx context.Context) (Interface, bool) {
	return kubeclient.FromContext(ctx)
}

// ApplyOptions returns how a reconciler writes a child resource itself, with
// kubewright's Create or Apply: as the inrun-templates field manager (the
// templates, and Go hooks that write children directly), forcing conflicts as
// the resource says (resourceForce), else as the CRD says, else true. A
// Result's children are applied as inrun-reconciler instead.
func ApplyOptions(c Interface, resourceForce *bool) kwtypes.Options {
	force := true
	switch {
	case resourceForce != nil:
		force = *resourceForce
	case c != nil && c.ForceConflict() != nil:
		force = *c.ForceConflict()
	}
	return kwtypes.Options{FieldManager: config.FieldManagerTemplates, Force: force}
}
