//go:build !runtime && !gateway

package structured

import (
	"strings"
	"testing"
)

func TestPruneEmptyYAML_RemovesEmptyString(t *testing.T) {
	input := `name: hello
empty: ""
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "empty") {
		t.Errorf("expected 'empty' key to be pruned, got:\n%s", out)
	}
	if !strings.Contains(string(out), "name: hello") {
		t.Errorf("expected 'name' key to be retained, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_RemovesZero(t *testing.T) {
	input := `replicas: 3
timeout: "0"
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "timeout") {
		t.Errorf("expected 'timeout: 0' to be pruned, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_RemovesZeroDuration(t *testing.T) {
	input := `name: foo
interval: 0s
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "interval") {
		t.Errorf("expected 'interval: 0s' to be pruned, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_RemovesNull(t *testing.T) {
	input := `name: foo
missing: null
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "missing") {
		t.Errorf("expected 'missing: null' to be pruned, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_RemovesEmptySequence(t *testing.T) {
	input := `name: foo
tags: []
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "tags") {
		t.Errorf("expected empty 'tags' sequence to be pruned, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_RemovesEmptyMapping(t *testing.T) {
	input := `name: foo
config: {}
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "config") {
		t.Errorf("expected empty 'config' mapping to be pruned, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_KeepsNonEmptySequence(t *testing.T) {
	input := `tags:
  - a
  - b
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "tags") {
		t.Errorf("expected non-empty 'tags' to be retained, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_KeepsNonZeroScalar(t *testing.T) {
	input := `replicas: 3
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "replicas") {
		t.Errorf("expected 'replicas: 3' to be retained, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_RecursesIntoNestedMapping(t *testing.T) {
	input := `spec:
  name: app
  empty: ""
  count: 0
`
	out, err := PruneEmptyYAML([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "empty") {
		t.Errorf("expected nested 'empty' to be pruned, got:\n%s", out)
	}
	if strings.Contains(string(out), "count") {
		t.Errorf("expected nested 'count: 0' to be pruned, got:\n%s", out)
	}
	if !strings.Contains(string(out), "name: app") {
		t.Errorf("expected nested 'name' to be retained, got:\n%s", out)
	}
}

func TestPruneEmptyYAML_InvalidYAML(t *testing.T) {
	_, err := PruneEmptyYAML([]byte("{{{not yaml"))
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestPruneEmptyYAML_EmptyInput(t *testing.T) {
	out, err := PruneEmptyYAML([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" && string(out) != "null\n" {
		// yaml.Marshal on empty doc may produce "null\n" — both are acceptable
		t.Logf("empty input produced: %q", out)
	}
}

// ── isEmptyValue ──────────────────────────────────────────────────────────────

func TestIsEmptyValue_Nil(t *testing.T) {
	if !isEmptyValue(nil) {
		t.Error("nil node should be empty")
	}
}
