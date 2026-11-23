package merger

import (
	"path/filepath"
	"testing"

	"github.com/intentd/tools/structured/structuredtest"
)

// The merger leaves a prepare include as written, for the Blueprint's
// include pass to expand; the CRD's own apply paths resolve from the
// Blueprint.
func TestLoadBlueprint_PrepareIncludeLeftForExpansion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blueprint.yaml")
	structuredtest.WriteFile(t, path, `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: includes
spec:
  crds:
    website:
      apiTypes:
        kind: Website
        group: example.io
      prepare:
        apply:
          - ./own.yaml
          - include: ./prepare/apply.yaml
`)
	m := New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}
	p := m.All()["website"].Prepare
	if p == nil || len(p.Apply) != 2 {
		t.Fatalf("prepare: %+v", p)
	}
	if p.Apply[0].Path != filepath.Join(dir, "own.yaml") {
		t.Errorf("own entry: %q", p.Apply[0].Path)
	}
	if p.Apply[1].Include != "./prepare/apply.yaml" || p.Apply[1].Path != "" {
		t.Errorf("include entry changed: %+v", p.Apply[1])
	}
}
