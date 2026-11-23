package e2e

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/intentd/jig"
	"github.com/intentd/tools/structured/structuredtest"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	structuredtest.WriteFile(t, path, content)
	return path
}

// A test without a blueprint maps to plain jig: no Inrun steps, the CR
// applied and deleted as steps, each after: remapped to its step. Mapping
// asks no cluster anything.
func TestToJig_PlainTestMapsCRLifecycle(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "cr.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n")
	path := write(t, dir, "e2e.yaml", `apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: plain
spec:
  cr: cr.yaml
  expect:
    - name: setup
      commands:
        - run: "true"
    - name: applied
      after: cr-applied
      commands:
        - run: "true"
    - name: deleted
      after: cr-deleted
      commands:
        - run: "true"
`)
	r, err := New(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s, cleanup, err := r.ToJig(context.Background())
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, st := range s.Prepare {
		names = append(names, st.Name)
	}
	if len(names) != 2 || names[0] != "cr" || names[1] != "cr-deleted" {
		t.Fatalf("steps = %v, want [cr cr-deleted]", names)
	}
	want := map[string]jig.After{"setup": jig.AfterEnvironment, "applied": "cr", "deleted": "cr-deleted"}
	for _, x := range s.Expect {
		if x.After != want[x.Name] {
			t.Errorf("%s: after = %q, want %q", x.Name, x.After, want[x.Name])
		}
	}
}

// inrun e2e runs a plain jig.yaml as it is.
func TestRunFromFile_PlainJigTest(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	path := write(t, dir, "jig.yaml", `name: plain-jig
environment:
  cluster:
    provider: none
expect:
  - name: runs
    timeout: 5s
    commands:
      - run: touch `+marker+`
`)
	if _, err := RunFromFile(context.Background(), path, Options{}); err != nil {
		t.Fatal(err)
	}
	if !exists(marker) {
		t.Fatal("the expectation did not run")
	}
}

// The dev server goes after the bundle, which creates its namespace: an
// aggregator (imports only) gets no dev-server step of its own.
func TestMap_NoDevServerWithoutBlueprint(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "child.yaml", "apiVersion: inrun.dev/v1\nkind: E2E\nmetadata:\n  name: child\nspec:\n  environment:\n    cluster:\n      provider: none\n  expect:\n    - name: e\n      commands:\n        - run: \"true\"\n")
	path := write(t, dir, "e2e.yaml", "apiVersion: inrun.dev/v1\nkind: E2E\nmetadata:\n  name: suite\nimports:\n  - ./child.yaml\n")
	r, err := New(path, Options{DevServer: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Map()
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range s.Prepare {
		if st.Name == stepDevServer {
			t.Fatalf("the aggregator got a %s step: %+v", stepDevServer, s.Prepare)
		}
	}
}

// Expectations keep the file's order: one without after: written after a
// cr-applied one runs with the CR applied, as http-auth's wrong-token checks
// do, not back before it.
func TestMap_AfterMovesForwardOnly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "cr.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n")
	path := write(t, dir, "e2e.yaml", `apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: order
spec:
  cr: cr.yaml
  expect:
    - name: before
      commands: [{run: "true"}]
    - name: applied
      after: cr-applied
      commands: [{run: "true"}]
    - name: still applied
      commands: [{run: "true"}]
    - name: setup again
      after: setup-complete
      commands: [{run: "true"}]
    - name: deleted
      after: cr-deleted
      commands: [{run: "true"}]
    - name: after deleted
      commands: [{run: "true"}]
`)
	r, err := New(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Map()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]jig.After{
		"before": jig.AfterEnvironment, "applied": "cr", "still applied": "cr",
		"setup again": "cr", "deleted": "cr-deleted", "after deleted": "cr-deleted",
	}
	for _, x := range s.Expect {
		if x.After != want[x.Name] {
			t.Errorf("%s: after = %q, want %q", x.Name, x.After, want[x.Name])
		}
	}
}

// prepare's apply and helm expand their includes through jig's types, each
// path resolved from the file that holds it, before they become steps.
func TestMap_PrepareIncludes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "include/helm/cert-manager.yaml", "helm:\n  - { repo: https://charts.jetstack.io, chart: cert-manager, valueFiles: [./values.yaml] }\n")
	write(t, dir, "include/setup.yaml", "apply:\n  - ./namespaces.yaml\n")
	path := write(t, dir, "e2e.yaml", `apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: includes
spec:
  prepare:
    apply:
      - include: ./include/setup.yaml
    helm:
      - include: ./include/helm/cert-manager.yaml
  expect:
    - name: e
      commands: [{run: "true"}]
`)
	r, err := New(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Map()
	if err != nil {
		t.Fatal(err)
	}
	steps := map[string]jig.Step{}
	for _, st := range s.Prepare {
		steps[st.Name] = st
	}
	apply := steps["prepare-apply"].Apply
	if len(apply) != 1 || apply[0].Path != filepath.Join(dir, "include", "namespaces.yaml") {
		t.Fatalf("prepare-apply: %+v", apply)
	}
	helm := steps["prepare-helm-1"].Helm
	if helm == nil || helm.Chart != "cert-manager" || helm.ValueFiles[0] != filepath.Join(dir, "include", "helm", "values.yaml") {
		t.Fatalf("prepare-helm-1: %+v", helm)
	}
}
