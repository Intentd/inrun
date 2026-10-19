//go:build integration

// tests/integration/stack/merge_test.go
// Integration tests for the Merger: loading Blueprint and Stack files from disk,
// resolving sources, merging CRD lists, and enforcing deduplication rules.
package stack_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/merger"
)

const blueprintYAML = `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test-blueprint
spec:
  crds:
    website:
      enabled: true
    database:
      enabled: true
`

const blueprintDisabledYAML = `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: test-disabled-blueprint
spec:
  crds:
    website:
      enabled: false
    cache:
      enabled: true
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "blueprint-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestMerger_SingleBlueprint_LoadsAllCRDs(t *testing.T) {
	path := writeTemp(t, blueprintYAML)
	m := merger.New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	if m.Count() != 2 {
		t.Errorf("expected 2 CRDs, got %d", m.Count())
	}
}

func TestMerger_DisabledCRD_ExcludedFromEnabled(t *testing.T) {
	path := writeTemp(t, blueprintDisabledYAML)
	m := merger.New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	if m.Count() != 2 {
		t.Errorf("All() should return 2 (including disabled), got %d", m.Count())
	}
	if m.EnabledCount() != 1 {
		t.Errorf("Enabled() should return 1, got %d", m.EnabledCount())
	}
}

func TestMerger_GetCRD_ByName(t *testing.T) {
	path := writeTemp(t, blueprintYAML)
	m := merger.New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	entry, ok := m.Get("website")
	if !ok {
		t.Fatal("expected to find 'website'")
	}
	if entry.Name != "website" {
		t.Errorf("expected name 'website', got %q", entry.Name)
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("should not find 'nonexistent'")
	}
}

func TestMerger_TwoBlueprintFiles_Merged(t *testing.T) {
	a := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: blueprint-a
spec:
  crds:
    alpha:
      enabled: true
`)
	b := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: blueprint-b
spec:
  crds:
    beta:
      enabled: true
`)
	m := merger.New(a, b)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	if m.Count() != 2 {
		t.Errorf("expected 2 CRDs from two blueprint files, got %d", m.Count())
	}
}

func TestMerger_DuplicateCRD_AcrossEntryPoints_ReturnsError(t *testing.T) {
	a := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: blueprint-a
spec:
  crds:
    website:
      enabled: true
`)
	b := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: blueprint-b
spec:
  crds:
    website:
      enabled: true
`)
	m := merger.New(a, b)
	if err := m.Merge(); err == nil {
		t.Error("expected error for duplicate CRD 'website' across entry points")
	}
}

func TestMerger_StackFileSource_LoadsBlueprint(t *testing.T) {
	blueprintPath := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: source-blueprint
spec:
  crds:
    sourced-crd:
      enabled: true
`)
	stackContent := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: test-stack\nimports:\n  files:\n    - url: " + blueprintPath + "\nspec:\n  crds: {}\n"
	stackPath := writeTemp(t, stackContent)

	m := merger.New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	if m.Count() != 1 {
		t.Errorf("expected 1 CRD from sourced Blueprint, got %d", m.Count())
	}
	if _, ok := m.Get("sourced-crd"); !ok {
		t.Error("expected to find 'sourced-crd'")
	}
}

func TestMerger_Add_AppendsEntryPoints(t *testing.T) {
	a := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: blueprint-a
spec:
  crds:
    alpha:
      enabled: true
`)
	b := writeTemp(t, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: blueprint-b
spec:
  crds:
    beta:
      enabled: true
`)
	m := merger.New(a).Add(b)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	if m.Count() != 2 {
		t.Errorf("expected 2 CRDs after Add(), got %d", m.Count())
	}
}

func TestMerger_NonExistentFile_ReturnsError(t *testing.T) {
	m := merger.New(filepath.Join(os.TempDir(), "does-not-exist-xyz.yaml"))
	if err := m.Merge(); err == nil {
		t.Error("expected error loading non-existent file")
	}
}
