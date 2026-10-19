// pkg/merger/profiles_test.go
package merger

import (
	"strings"
	"testing"
)

const blueprintWithProfilesYAML = `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: profiled
profiles:
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

// TestBlueprint_InlineProfiles_ForwardedToToProfiles verifies that profiles:
// declared directly in a Blueprint are returned by ToProfiles() after Merge().
func TestBlueprint_InlineProfiles_ForwardedToToProfiles(t *testing.T) {
	dir := t.TempDir()
	path := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithProfilesYAML)

	m := New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	profiles := m.ToProfiles()
	if len(profiles.Reconciler) != 2 {
		t.Fatalf("expected 2 reconciler profiles, got %d", len(profiles.Reconciler))
	}
	if _, ok := profiles.LookupReconciler("fast"); !ok {
		t.Error("expected reconciler profile 'fast'")
	}
	if _, ok := profiles.LookupReconciler("slow"); !ok {
		t.Error("expected reconciler profile 'slow'")
	}
}

// TestStack_InlineProfiles_MergedWithBlueprintProfiles verifies that a Stack
// can add profiles with names that do not conflict with imported Blueprint profiles,
// and that both sets appear in ToProfiles().
func TestStack_InlineProfiles_MergedWithBlueprintProfiles(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithProfilesYAML)

	// Stack declares a profile with a different name — no conflict.
	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\nprofiles:\n  reconciler:\n    - name: batch\n      workers: 5\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	profiles := m.ToProfiles()
	if _, ok := profiles.LookupReconciler("fast"); !ok {
		t.Error("expected blueprint profile 'fast' to pass through")
	}
	if _, ok := profiles.LookupReconciler("slow"); !ok {
		t.Error("expected blueprint profile 'slow' to pass through")
	}
	if _, ok := profiles.LookupReconciler("batch"); !ok {
		t.Error("expected Stack profile 'batch'")
	}
}

// TestStack_InlineProfiles_ConflictErrors verifies that a Stack declaring
// a profile with the same name as one from an imported Blueprint returns an error.
// Unlike notes, profiles use conflict-detection rather than last-wins.
func TestStack_InlineProfiles_ConflictErrors(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithProfilesYAML)

	// 'fast' is also declared in the Blueprint — this must error.
	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\nprofiles:\n  reconciler:\n    - name: fast\n      workers: 99\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	err := m.Merge()
	if err == nil {
		t.Fatal("expected conflict error for duplicate profile name 'fast', got nil")
	}
	if !strings.Contains(err.Error(), "fast") {
		t.Errorf("expected error to mention the conflicting profile name 'fast', got: %v", err)
	}
}
