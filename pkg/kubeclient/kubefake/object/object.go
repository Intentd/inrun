// Package object builds the unstructured objects tests need, as kubefake
// builds the client they talk to. It is for tests and simulate fixtures only,
// and imports nothing from Inrun, so any package's tests can use it.
//
//	cr := object.New("demo.inrun.dev/v1", "Website", "web",
//		object.InNamespace("team"),
//		object.WithSpec(map[string]any{"domain": "example.com"}))
package object

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// The owner Owner and ClusterOwner build.
const (
	OwnerAPIVersion = "demo.inrun.dev/v1"
	OwnerKind       = "App"
	OwnerName       = "app"
	OwnerNamespace  = "default"
	OwnerUID        = "app-uid"
)

// Option sets one part of an object.
type Option func(*unstructured.Unstructured)

// New builds an object of apiVersion and kind named name, then applies opts.
// An empty apiVersion, kind or name is left unset, for tests that need an
// object with only some fields.
func New(apiVersion, kind, name string, opts ...Option) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{}}
	if apiVersion != "" {
		u.SetAPIVersion(apiVersion)
	}
	if kind != "" {
		u.SetKind(kind)
	}
	if name != "" {
		u.SetName(name)
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// Empty returns an object with no fields set.
func Empty(opts ...Option) *unstructured.Unstructured {
	return New("", "", "", opts...)
}

// Owner returns a namespaced custom resource to own children:
// demo.inrun.dev/v1 App "app" in "default", UID "app-uid".
func Owner(opts ...Option) *unstructured.Unstructured {
	return New(OwnerAPIVersion, OwnerKind, OwnerName,
		append([]Option{InNamespace(OwnerNamespace), WithUID(OwnerUID)}, opts...)...)
}

// ClusterOwner returns Owner without a namespace.
func ClusterOwner(opts ...Option) *unstructured.Unstructured {
	return New(OwnerAPIVersion, OwnerKind, OwnerName, append([]Option{WithUID(OwnerUID)}, opts...)...)
}

// InNamespace sets the namespace.
func InNamespace(ns string) Option {
	return func(u *unstructured.Unstructured) { u.SetNamespace(ns) }
}

// WithUID sets the UID.
func WithUID(uid string) Option {
	return func(u *unstructured.Unstructured) { u.SetUID(k8stypes.UID(uid)) }
}

// WithResourceVersion sets the resourceVersion.
func WithResourceVersion(rv string) Option {
	return func(u *unstructured.Unstructured) { u.SetResourceVersion(rv) }
}

// WithGeneration sets the generation.
func WithGeneration(generation int64) Option {
	return func(u *unstructured.Unstructured) { u.SetGeneration(generation) }
}

// WithLabels sets the labels.
func WithLabels(labels map[string]string) Option {
	return func(u *unstructured.Unstructured) { u.SetLabels(labels) }
}

// WithAnnotations sets the annotations.
func WithAnnotations(annotations map[string]string) Option {
	return func(u *unstructured.Unstructured) { u.SetAnnotations(annotations) }
}

// WithFinalizers sets the finalizers.
func WithFinalizers(finalizers ...string) Option {
	return func(u *unstructured.Unstructured) { u.SetFinalizers(finalizers) }
}

// Deleting sets the deletion timestamp to at.
func Deleting(at time.Time) Option {
	return func(u *unstructured.Unstructured) {
		ts := metav1.NewTime(at)
		u.SetDeletionTimestamp(&ts)
	}
}

// ManagedBy records an apply by manager in managedFields.
func ManagedBy(manager string) Option {
	return func(u *unstructured.Unstructured) {
		u.SetManagedFields(append(u.GetManagedFields(), metav1.ManagedFieldsEntry{
			Manager:   manager,
			Operation: metav1.ManagedFieldsOperationApply,
		}))
	}
}

// WithSpec sets spec.
func WithSpec(spec map[string]any) Option {
	return WithField(spec, "spec")
}

// WithStatus sets status.
func WithStatus(status map[string]any) Option {
	return WithField(status, "status")
}

// WithField sets the field at path to value, creating the maps on the way.
// value is stored as given: build it from the types unstructured content
// allows (map[string]any, []any, string, int64, float64, bool).
func WithField(value any, path ...string) Option {
	return func(u *unstructured.Unstructured) {
		m := u.Object
		for _, key := range path[:len(path)-1] {
			next, ok := m[key].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[key] = next
			}
			m = next
		}
		m[path[len(path)-1]] = value
	}
}
