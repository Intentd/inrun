package expect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// ── ExpandIncludes ──────────────────────────────────────────────────────

func TestExpandIncludes_NoIncludes(t *testing.T) {
	expects := []types.E2EExpectation{
		{Name: "a"},
		{Name: "b"},
	}
	got, err := ExpandIncludes(expects, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestExpandIncludes_ExpandsInPlace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "extra.yaml"), `
expect:
  - name: x
  - name: y
`)
	expects := []types.E2EExpectation{
		{Name: "before"},
		{Include: "./extra.yaml"},
		{Name: "after"},
	}
	got, err := ExpandIncludes(expects, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d: %v", len(got), got)
	}
	names := []string{got[0].Name, got[1].Name, got[2].Name, got[3].Name}
	want := []string{"before", "x", "y", "after"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("index %d: got %q, want %q", i, n, want[i])
		}
	}
}

func TestExpandIncludes_MultipleIncludes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.yaml"), `
expect:
  - name: a1
  - name: a2
`)
	writeFile(t, filepath.Join(dir, "b.yaml"), `
expect:
  - name: b1
`)
	expects := []types.E2EExpectation{
		{Include: "./a.yaml"},
		{Name: "mid"},
		{Include: "./b.yaml"},
	}
	got, err := ExpandIncludes(expects, dir)
	if err != nil {
		t.Fatal(err)
	}
	names := names(got)
	want := []string{"a1", "a2", "mid", "b1"}
	if !stringSliceEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestExpandIncludes_MissingExpectKeyYieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	// File exists but has no 'expect:' key — should expand to nothing, no error.
	writeFile(t, filepath.Join(dir, "empty.yaml"), `
metadata:
  name: something
`)
	expects := []types.E2EExpectation{{Include: "./empty.yaml"}}
	got, err := ExpandIncludes(expects, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty expansion, got %v", got)
	}
}

func TestExpandIncludes_MissingFile(t *testing.T) {
	dir := t.TempDir()
	expects := []types.E2EExpectation{{Include: "./nonexistent.yaml"}}
	_, err := ExpandIncludes(expects, dir)
	if err == nil {
		t.Fatal("expected error for missing include file, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent.yaml") {
		t.Errorf("error should mention the file name, got: %v", err)
	}
}

func TestExpandIncludes_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), `{{{not yaml`)
	expects := []types.E2EExpectation{{Include: "./bad.yaml"}}
	_, err := ExpandIncludes(expects, dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestExpandIncludes_AbsoluteIncludePath(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "abs.yaml")
	writeFile(t, absFile, `
expect:
  - name: abs-entry
`)
	expects := []types.E2EExpectation{{Include: absFile}}
	got, err := ExpandIncludes(expects, "/unrelated/dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "abs-entry" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestExpandIncludes_WorkDirIsIncludeFileDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "steps.yaml"), `
expect:
  - name: step-a
  - name: step-b
`)
	expects := []types.E2EExpectation{{Include: filepath.Join(sub, "steps.yaml")}}
	got, err := ExpandIncludes(expects, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 expectations, got %d", len(got))
	}
	for _, exp := range got {
		if exp.WorkDir != sub {
			t.Errorf("expected WorkDir %q, got %q", sub, exp.WorkDir)
		}
	}
}
