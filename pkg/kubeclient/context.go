// pkg/kubeclient/context.go
package kubeclient

import "context"

// contextKey is an unexported type for context keys in this package.
type contextKey string

// ContextKey is the key used to store and retrieve a Interface from a context.
//
// Usage — inject before calling hooks:
//
//	ctx = kubeclient.WithKubeclient(ctx, kube)
//
// Usage — retrieve in hook/registry functions:
//
//	kube, ok := kubeclient.FromContext(ctx)
const ContextKey contextKey = "inrun-kubeclient"

// WithKubeclient returns a new context with the Interface stored under ContextKey.
// Called in generic.Reconciler.Reconcile before invoking hook and registry functions.
func WithKubeclient(ctx context.Context, kube Interface) context.Context {
	return context.WithValue(ctx, ContextKey, kube)
}

// FromContext retrieves the Interface from a context.
// Returns (nil, false) if not present.
func FromContext(ctx context.Context) (Interface, bool) {
	kube, ok := ctx.Value(ContextKey).(Interface)
	return kube, ok && kube != nil
}

// patchPartKey stores which part of an object a kubeclient patch writes.
const patchPartKey contextKey = "inrun-patch-part"

// Patch parts, named as the fake client records them.
const (
	PatchPartFinalizers  = "finalizers"
	PatchPartLabels      = "labels"
	PatchPartAnnotations = "annotations"
	PatchPartStatus      = "status"
	PatchPartSpec        = "spec"
)

// withPatchPart tags ctx with the part a patch writes. PatchFinalizers,
// PatchLabels, PatchAnnotations, PatchStatus and PatchSpec set it; client-go carries
// ctx onto the HTTP request, where a recording transport reads it back.
func withPatchPart(ctx context.Context, part string) context.Context {
	return context.WithValue(ctx, patchPartKey, part)
}

// PatchPartFrom returns the part a request's patch writes, as tagged by the
// kubeclient method that sent it, or "" for any other request.
func PatchPartFrom(ctx context.Context) string {
	part, _ := ctx.Value(patchPartKey).(string)
	return part
}
