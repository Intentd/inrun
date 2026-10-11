package maintain

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func makeObj(finalizers ...string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetFinalizers(finalizers)
	return obj
}

// ── ContainsFinalizer ─────────────────────────────────────────────────────────

func TestContainsFinalizer_Present(t *testing.T) {
	obj := makeObj("inrun.dev/cleanup", "other.io/finalizer")
	if !labels.ContainsFinalizer(obj, "inrun.dev/cleanup") {
		t.Error("expected finalizer to be found")
	}
}

func TestContainsFinalizer_Absent(t *testing.T) {
	obj := makeObj("other.io/finalizer")
	if labels.ContainsFinalizer(obj, "inrun.dev/cleanup") {
		t.Error("expected finalizer to be absent")
	}
}

func TestContainsFinalizer_Empty(t *testing.T) {
	obj := makeObj()
	if labels.ContainsFinalizer(obj, "inrun.dev/cleanup") {
		t.Error("no finalizers — must return false")
	}
}

// ── AddFinalizer ──────────────────────────────────────────────────────────────

func TestAddFinalizer_AddsWhenAbsent(t *testing.T) {
	obj := makeObj()
	updated := labels.AddFinalizer(obj, "inrun.dev/cleanup")
	if !updated {
		t.Error("expected updated=true when adding a new finalizer")
	}
	if !labels.ContainsFinalizer(obj, "inrun.dev/cleanup") {
		t.Error("finalizer must be present after Add")
	}
}

func TestAddFinalizer_NoopWhenPresent(t *testing.T) {
	obj := makeObj("inrun.dev/cleanup")
	updated := labels.AddFinalizer(obj, "inrun.dev/cleanup")
	if updated {
		t.Error("expected updated=false when finalizer already present")
	}
	if len(obj.GetFinalizers()) != 1 {
		t.Error("finalizer must not be duplicated")
	}
}

func TestAddFinalizer_MultipleFinalizers(t *testing.T) {
	obj := makeObj("first.io/f")
	labels.AddFinalizer(obj, "second.io/f")
	labels.AddFinalizer(obj, "third.io/f")
	f := obj.GetFinalizers()
	if len(f) != 3 {
		t.Errorf("expected 3 finalizers, got %d: %v", len(f), f)
	}
}

// ── RemoveFinalizer ───────────────────────────────────────────────────────────

func TestRemoveFinalizer_RemovesWhenPresent(t *testing.T) {
	obj := makeObj("inrun.dev/cleanup", "other.io/f")
	updated := labels.RemoveFinalizer(obj, "inrun.dev/cleanup")
	if !updated {
		t.Error("expected updated=true when removing a present finalizer")
	}
	if labels.ContainsFinalizer(obj, "inrun.dev/cleanup") {
		t.Error("finalizer must be gone after Remove")
	}
	if !labels.ContainsFinalizer(obj, "other.io/f") {
		t.Error("other finalizer must remain")
	}
}

func TestRemoveFinalizer_NoopWhenAbsent(t *testing.T) {
	obj := makeObj("other.io/f")
	updated := labels.RemoveFinalizer(obj, "inrun.dev/cleanup")
	if updated {
		t.Error("expected updated=false when finalizer not present")
	}
	if len(obj.GetFinalizers()) != 1 {
		t.Error("existing finalizer must remain unchanged")
	}
}

func TestRemoveFinalizer_EmptyList(t *testing.T) {
	obj := makeObj()
	updated := labels.RemoveFinalizer(obj, "inrun.dev/cleanup")
	if updated {
		t.Error("expected updated=false on empty finalizer list")
	}
}

func TestRemoveFinalizer_AllRemoved(t *testing.T) {
	obj := makeObj("inrun.dev/cleanup")
	labels.RemoveFinalizer(obj, "inrun.dev/cleanup")
	if len(obj.GetFinalizers()) != 0 {
		t.Error("all finalizers should be removed")
	}
}

func TestRemoveFinalizer_OnlyTargetRemoved(t *testing.T) {
	obj := makeObj("a", "b", "c")
	labels.RemoveFinalizer(obj, "b")
	f := obj.GetFinalizers()
	if len(f) != 2 {
		t.Errorf("expected 2 remaining, got %v", f)
	}
	for _, fin := range f {
		if fin == "b" {
			t.Error("b must be removed")
		}
	}
}

// ── Finalize and ensureFinalizers ─────────────────────────────────────────────

// patchRecorder records PatchFinalizers calls; nothing else is used.
type patchRecorder struct {
	kubeclient.Interface
	calls int
	got   []string
	err   error
}

func (p *patchRecorder) PatchFinalizers(_ context.Context, _ runtime.Object, f []string, _ metav1.PatchOptions) error {
	p.calls++
	p.got = f
	return p.err
}

func cleanupBox() types.OperatorBoxConfig {
	return types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{Finalizers: &types.FinalizerConfig{
		Names: []string{labels.CleanupFinalizer},
	}}}
}

func TestFinalize_RemovesOnlyInrunsFinalizers(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetFinalizers([]string{labels.CleanupFinalizer, "other.io/finalizer"})
	kube := &patchRecorder{}

	require.NoError(t, Finalize(context.Background(), kube, obj, cleanupBox()))
	assert.Equal(t, 1, kube.calls)
	assert.Equal(t, []string{"other.io/finalizer"}, kube.got)
}

func TestFinalize_NoPatchWithoutInrunsFinalizer(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetFinalizers([]string{"other.io/finalizer"})
	kube := &patchRecorder{}

	require.NoError(t, Finalize(context.Background(), kube, obj, cleanupBox()))
	assert.Zero(t, kube.calls)
}

func TestEnsureFinalizers_SkipsObjectBeingDeleted(t *testing.T) {
	obj := &unstructured.Unstructured{}
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)
	obj.SetFinalizers([]string{"other.io/finalizer"})
	kube := &patchRecorder{}

	require.NoError(t, ensureFinalizers(context.Background(), Input{Kube: kube}, obj, cleanupBox()))
	assert.Zero(t, kube.calls, "the API server refuses new finalizers on a deleting object")
}

func TestFinalize_AlreadyGoneIsDone(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetName("gone")
	obj.SetFinalizers([]string{labels.CleanupFinalizer})
	kube := &patchRecorder{err: apierrors.NewNotFound(schema.GroupResource{Resource: "webapps"}, "gone")}

	assert.NoError(t, Finalize(context.Background(), kube, obj, cleanupBox()))
}
