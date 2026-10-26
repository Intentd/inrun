// pkg/merger/merger_test.go
package merger

import (
	"os"
	"path/filepath"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func writeTempBlueprint(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// ── upstream Blueprint fields flow through Stack ────────────────────────────

const upstreamBlueprintYAML = `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: upstream
security:
  serviceName:
    runtime: upstream-svc
spec:
  crds:
    website:
      apiTypes:
        kind: Website
        group: example.io
`

const stackYAML = `apiVersion: inrun.dev/v1
kind: Stack
metadata:
  name: my-stack
imports:
  files:
    - url: %s
`

func TestStack_InheritsUpstreamSecurity(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "upstream.yaml", upstreamBlueprintYAML)

	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: my-stack\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	sec := m.ToSecurity()
	if sec.ServiceName == nil || sec.ServiceName.Runtime != "upstream-svc" {
		t.Errorf("expected security.serviceName.runtime=upstream-svc, got %v", sec.ServiceName)
	}
}

func TestStack_OwnSecurityWinsOverUpstream(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "upstream.yaml", upstreamBlueprintYAML)

	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: my-stack\nsecurity:\n  serviceName:\n    runtime: stack-svc\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	sec := m.ToSecurity()
	if sec.ServiceName == nil || sec.ServiceName.Runtime != "stack-svc" {
		t.Errorf("expected stack serviceName.runtime to win, got %v", sec.ServiceName)
	}
}

func TestStack_CRDsFromUpstreamBlueprint(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "upstream.yaml", upstreamBlueprintYAML)

	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: my-stack\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	crds := m.All()
	if _, ok := crds["website"]; !ok {
		t.Error("expected website CRD from upstream Blueprint")
	}
}
