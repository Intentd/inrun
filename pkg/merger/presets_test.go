// pkg/merger/presets_test.go
package merger

import (
	"strings"
	"testing"
)

const blueprintWithPresetsYAML = `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: presetd
presets:
  reconciler:
    - name: fast
      workers: 10
    - name: slow
      workers: 1
spec:
  crds:
    widget:
      apiTypes:
        kind: Widget
        group: example.io
`

// TestBlueprint_InlinePresets_ForwardedToToPresets verifies that presets:
// declared directly in a Blueprint are returned by ToPresets() after Merge().
func TestBlueprint_InlinePresets_ForwardedToToPresets(t *testing.T) {
	dir := t.TempDir()
	path := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithPresetsYAML)

	m := New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	presets := m.ToPresets()
	if len(presets.Reconciler) != 2 {
		t.Fatalf("expected 2 reconciler presets, got %d", len(presets.Reconciler))
	}
	if _, ok := presets.LookupReconciler("fast"); !ok {
		t.Error("expected reconciler preset 'fast'")
	}
	if _, ok := presets.LookupReconciler("slow"); !ok {
		t.Error("expected reconciler preset 'slow'")
	}
}

// TestStack_InlinePresets_MergedWithBlueprintPresets verifies that a Stack
// can add presets with names that do not conflict with imported Blueprint presets,
// and that both sets appear in ToPresets().
func TestStack_InlinePresets_MergedWithBlueprintPresets(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithPresetsYAML)

	// Stack declares a preset with a different name — no conflict.
	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\npresets:\n  reconciler:\n    - name: batch\n      workers: 5\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	presets := m.ToPresets()
	if _, ok := presets.LookupReconciler("fast"); !ok {
		t.Error("expected blueprint preset 'fast' to pass through")
	}
	if _, ok := presets.LookupReconciler("slow"); !ok {
		t.Error("expected blueprint preset 'slow' to pass through")
	}
	if _, ok := presets.LookupReconciler("batch"); !ok {
		t.Error("expected Stack preset 'batch'")
	}
}

// TestStack_InlinePresets_ConflictErrors verifies that a Stack declaring
// a preset with the same name as one from an imported Blueprint returns an error.
// Unlike notes, presets use conflict-detection rather than last-wins.
func TestStack_InlinePresets_ConflictErrors(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithPresetsYAML)

	// 'fast' is also declared in the Blueprint — this must error.
	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\npresets:\n  reconciler:\n    - name: fast\n      workers: 99\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	err := m.Merge()
	if err == nil {
		t.Fatal("expected conflict error for duplicate preset name 'fast', got nil")
	}
	if !strings.Contains(err.Error(), "fast") {
		t.Errorf("expected error to mention the conflicting preset name 'fast', got: %v", err)
	}
}
