package maintain_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
