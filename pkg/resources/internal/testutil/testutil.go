// Package testutil holds what the kind packages' tests share: an owner to
// build for, and a check of the verbs' results against a fake clientset.
package testutil

import (
	"context"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// OwnerUID is Owner's UID.
const OwnerUID = "11111111-2222-3333-4444-555555555555"

// Owner returns the object the tests build for: an App named "app" in
// namespace "team".
func Owner() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("demo.example.com/v1")
	u.SetKind("App")
	u.SetName("app")
	u.SetNamespace("team")
	u.SetUID(OwnerUID)
	return u
}

// Lifecycle is one kind's verbs, bound to a spec, for Check.
type Lifecycle struct {
	Create func(ctx context.Context, cs kubernetes.Interface) (types.Result, error)
	// Apply is nil for kinds without one.
	Apply func(ctx context.Context, cs kubernetes.Interface) (types.Result, error)
	// ApplyResult is what Apply reports for an object Create just made:
	// Applied unless set.
	ApplyResult types.Result
	Delete      func(ctx context.Context, cs kubernetes.Interface) (types.Result, error)
	// DeleteIfOwned is nil for kinds without one.
	DeleteIfOwned func(ctx context.Context, cs kubernetes.Interface, owned func(metav1.Object) bool) (types.Result, error)
	// Get reads the object back.
	Get func(ctx context.Context, cs kubernetes.Interface) (metav1.Object, error)
}

// Check runs the verbs against a fake clientset and checks each result:
// Create is Created then Unchanged, Delete is Deleted then
// NotFound, and DeleteIfOwned leaves what the predicate does not own.
// Apply is Applied, or ApplyResult when set.
func (l Lifecycle) Check(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	cs := fake.NewClientset()

	want(t, "first Create", types.Created)(l.Create(ctx, cs))
	exists(t, ctx, cs, l.Get, true)
	want(t, "second Create", types.Unchanged)(l.Create(ctx, cs))

	if l.Apply != nil {
		res := l.ApplyResult
		if res == "" {
			res = types.Applied
		}
		want(t, "Apply", res)(l.Apply(ctx, cs))
		exists(t, ctx, cs, l.Get, true)
	}

	if l.DeleteIfOwned != nil {
		notOurs := func(metav1.Object) bool { return false }
		want(t, "DeleteIfOwned of another owner's", types.NotOwned)(l.DeleteIfOwned(ctx, cs, notOurs))
		exists(t, ctx, cs, l.Get, true)
	}

	want(t, "Delete", types.Deleted)(l.Delete(ctx, cs))
	exists(t, ctx, cs, l.Get, false)
	want(t, "second Delete", types.NotFound)(l.Delete(ctx, cs))

	if l.DeleteIfOwned != nil {
		ours := func(metav1.Object) bool { return true }
		want(t, "Create again", types.Created)(l.Create(ctx, cs))
		want(t, "DeleteIfOwned of ours", types.Deleted)(l.DeleteIfOwned(ctx, cs, ours))
		exists(t, ctx, cs, l.Get, false)
		want(t, "DeleteIfOwned of a deleted one", types.NotFound)(l.DeleteIfOwned(ctx, cs, ours))
	}
}

// want returns a check of a verb's (Result, error) against res.
func want(t *testing.T, step string, res types.Result) func(types.Result, error) {
	t.Helper()
	return func(got types.Result, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		if got != res {
			t.Fatalf("%s: result %q, want %q", step, got, res)
		}
	}
}

func exists(t *testing.T, ctx context.Context, cs kubernetes.Interface, get func(context.Context, kubernetes.Interface) (metav1.Object, error), want bool) {
	t.Helper()
	_, err := get(ctx, cs)
	switch {
	case want && err != nil:
		t.Fatalf("object should exist: %v", err)
	case !want && err == nil:
		t.Fatal("object should be gone")
	case !want && !k8serrors.IsNotFound(err):
		t.Fatalf("reading a deleted object: %v", err)
	}
}

// Owned reports whether obj has one owner reference, to Owner.
func Owned(obj metav1.Object) bool {
	refs := obj.GetOwnerReferences()
	return len(refs) == 1 && string(refs[0].UID) == OwnerUID && refs[0].Kind == "App" && refs[0].Name == "app"
}
