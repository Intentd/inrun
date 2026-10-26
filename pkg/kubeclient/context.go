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
