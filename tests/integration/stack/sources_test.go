//go:build integration

// tests/integration/stack/sources_test.go
// Integration tests for Stack source resolution and inline override behaviour.
package stack_test

import (
	"os"
	"testing"

	"github.com/intentd/inrun/pkg/merger"
)

// writeBlueprintFile creates a minimal Blueprint YAML temp file with the given CRD names.
func writeBlueprintFile(t *testing.T, name string, crdNames ...string) string {
	t.Helper()
	content := "apiVersion: inrun.dev/v1\nkind: Blueprint\nmetadata:\n  name: " + name + "\nspec:\n  crds:\n"
	for _, n := range crdNames {
		content += "    " + n + ":\n      enabled: true\n"
	}
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestStack_InlineOverride_WinsOverSource(t *testing.T) {
	sourceBlueprint := writeBlueprintFile(t, "source", "website", "database")

	stackContent := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: override-test\nimports:\n  files:\n    - url: " + sourceBlueprint + "\nspec:\n  crds:\n    website:\n      enabled: false\n"
	f, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.WriteString(stackContent)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	m := merger.New(f.Name())
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}

	if m.Count() != 2 {
		t.Errorf("expected 2 total CRDs, got %d", m.Count())
	}

	website, ok := m.Get("website")
	if !ok {
		t.Fatal("expected to find 'website'")
	}
	if website.IsEnabled() {
		t.Error("inline override should have disabled 'website'")
	}

	db, ok := m.Get("database")
	if !ok {
		t.Fatal("expected to find 'database'")
	}
	if !db.IsEnabled() {
		t.Error("'database' should remain enabled (not overridden)")
	}
}

func TestStack_MultipleFileSources_AllMerged(t *testing.T) {
	srcA := writeBlueprintFile(t, "source-a", "crd-a1", "crd-a2")
	srcB := writeBlueprintFile(t, "source-b", "crd-b1")

	content := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: multi-source\nimports:\n  files:\n    - url: " + srcA + "\n    - url: " + srcB + "\nspec:\n  crds: {}\n"
	f, _ := os.CreateTemp("", "*.yaml")
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	m := merger.New(f.Name())
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() failed: %v", err)
	}
	if m.Count() != 3 {
		t.Errorf("expected 3 CRDs from two sources, got %d", m.Count())
	}
}

func TestStack_CannotSourceAnotherStack(t *testing.T) {
	innerStack := `apiVersion: inrun.dev/v1
kind: Stack
metadata:
  name: inner-stack
sources:
  files: []
spec:
  crds:
    inner:
      enabled: true
`
	fInner, _ := os.CreateTemp("", "*.yaml")
	fInner.WriteString(innerStack)
	fInner.Close()
	t.Cleanup(func() { os.Remove(fInner.Name()) })

	outerContent := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: outer-stack\nsources:\n  files:\n    - url: " + fInner.Name() + "\nspec:\n  crds: {}\n"
	fOuter, _ := os.CreateTemp("", "*.yaml")
	fOuter.WriteString(outerContent)
	fOuter.Close()
	t.Cleanup(func() { os.Remove(fOuter.Name()) })

	m := merger.New(fOuter.Name())
	if err := m.Merge(); err == nil {
		t.Error("expected error: Stack cannot source another Stack")
	}
}

func TestStack_BlueprintCannotDeclareSourcesBlock(t *testing.T) {
	content := `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: bad-blueprint
sources:
  files:
    - url: ./some-file.yaml
spec:
  crds:
    website:
      enabled: true
`
	f, _ := os.CreateTemp("", "*.yaml")
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	m := merger.New(f.Name())
	if err := m.Merge(); err == nil {
		t.Error("expected error: kind Blueprint cannot declare sources block")
	}
}
