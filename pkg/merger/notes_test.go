// pkg/merger/notes_test.go
package merger

import (
	"strings"
	"testing"
)

const blueprintWithFunctionsYAML = `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: noted
functions:
  definitions:
    - name: host
      expression: "{{ .metadata.name }}.cluster.local"
    - name: img
      expression: "{{ .spec.image }}:latest"
spec:
  crds:
    widget:
      apiTypes:
        kind: Widget
        group: example.io
`

// TestBlueprint_InlineNotes_ForwardedToToNotes verifies that functions: declared
// directly in a Blueprint are returned by ToFunctions() after Merge().
func TestBlueprint_InlineNotes_ForwardedToToNotes(t *testing.T) {
	dir := t.TempDir()
	path := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithFunctionsYAML)

	m := New(path)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	funcs := m.ToFunctions()
	if len(funcs.Definitions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(funcs.Definitions))
	}
	names := map[string]bool{}
	for _, n := range funcs.Definitions {
		names[n.Name] = true
	}
	if !names["host"] {
		t.Error("expected function 'host' from Blueprint")
	}
	if !names["img"] {
		t.Error("expected function 'img' from Blueprint")
	}
}

// TestStack_InlineNotes_ForwardedToToNotes verifies that functions: declared on
// a Stack are visible via ToFunctions() after the Blueprint files are imported.
func TestStack_InlineNotes_ForwardedToToFunctions(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithFunctionsYAML)

	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\nfunctions:\n  definitions:\n    - name: env\n      expression: \"prod\"\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	funcs := m.ToFunctions()
	names := map[string]bool{}
	for _, n := range funcs.Definitions {
		names[n.Name] = true
	}
	if !names["env"] {
		t.Error("expected Stack inline note 'env'")
	}
	// Notes from the imported Blueprint must also be present.
	if !names["host"] {
		t.Error("expected Blueprint note 'host' to pass through")
	}
}

// TestStack_InlineNotes_OverrideBlueprintNote verifies that when a Stack
// declares a note with the same name as one in an imported Blueprint, the
// Stack's expression is the one returned (last-wins via FuncMap ordering).
func TestStack_InlineNotes_OverrideBlueprintNote(t *testing.T) {
	dir := t.TempDir()
	blueprintPath := writeTempBlueprint(t, dir, "blueprint.yaml", blueprintWithFunctionsYAML)

	// host is also declared in the Blueprint; Stack's value must win (appended last).
	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\nfunctions:\n  definitions:\n    - name: host\n      expression: \"{{ .metadata.name }}.prod.example.com\"\nimports:\n  files:\n    - url: " + blueprintPath + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}

	funcs := m.ToFunctions()
	// The Stack function is appended after Blueprint functions, so it appears last in the registry.
	// Confirm the last entry for 'host' comes from the Stack.
	var lastHost string
	for _, n := range funcs.Definitions {
		if n.Name == "host" {
			lastHost = n.Expression
		}
	}
	if !strings.Contains(lastHost, "prod.example.com") {
		t.Errorf("expected Stack's host expression to win, got %q", lastHost)
	}
}

// TestStack_CrossBlueprintNoteConflict_Errors verifies that two imported Blueprints
// declaring the same function name returns an error. Unlike the Stack's own functions:
// block (which intentionally overrides), cross-Blueprint conflicts are ambiguous and
// must be surfaced rather than silently resolved by import order.
func TestStack_CrossBlueprintNoteConflict_Errors(t *testing.T) {
	dir := t.TempDir()

	src1 := writeTempBlueprint(t, dir, "src1.yaml", `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: src1
functions:
  definitions:
    - name: host
      expression: "{{ .metadata.name }}.src1.local"
spec:
  crds:
    alpha:
      apiTypes:
        kind: Alpha
        group: example.io
`)
	src2 := writeTempBlueprint(t, dir, "src2.yaml", `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: src2
functions:
  definitions:
    - name: host
      expression: "{{ .metadata.name }}.src2.local"
spec:
  crds:
    beta:
      apiTypes:
        kind: Beta
        group: example.io
`)

	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\nimports:\n  files:\n    - url: " + src1 + "\n    - url: " + src2 + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	err := m.Merge()
	if err == nil {
		t.Fatal("expected conflict error for function 'host' declared in two Blueprints, got nil")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("expected error to mention conflicting function name 'host', got: %v", err)
	}
}

// TestStack_SpecImports_Rejected verifies that a Stack with spec.imports
// is rejected with an error. spec.imports is reserved for Blueprints only.
func TestStack_SpecImports_Rejected(t *testing.T) {
	dir := t.TempDir()

	// Stack has both spec.crds (passes the empty-stack guard) and spec.imports.
	stack := `apiVersion: inrun.dev/v1
kind: Stack
metadata:
  name: bad-stack
spec:
  imports:
    - module: ./some-module.yaml
  crds:
    widget:
      apiTypes:
        kind: Widget
        group: example.io
`
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	err := m.Merge()
	if err == nil {
		t.Fatal("expected error for Stack with spec.imports, got nil")
	}
	if !strings.Contains(err.Error(), "spec.imports") {
		t.Errorf("expected error to mention spec.imports, got: %v", err)
	}
}

// TestStack_SharedNotesInclude_Reused verifies that two imported Blueprints
// including the same notes file contribute one function, not a conflict, and
// that an imported Blueprint's functions.include is expanded against its own directory.
func TestStack_SharedNotesInclude_Reused(t *testing.T) {
	dir := t.TempDir()
	writeTempBlueprint(t, dir, "functions.yaml", `definitions:
  - name: inBusinessHours
    expression: '{{ and weekday (timeInWindow "09:00" "18:00") }}'
`)
	blueprint := func(name, kind string) string {
		return `apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: ` + name + `
functions:
  include: functions.yaml
spec:
  crds:
    ` + name + `:
      apiTypes:
        kind: ` + kind + `
        group: example.io
`
	}
	src1 := writeTempBlueprint(t, dir, "src1.yaml", blueprint("alpha", "Alpha"))
	src2 := writeTempBlueprint(t, dir, "src2.yaml", blueprint("beta", "Beta"))
	stack := "apiVersion: inrun.dev/v1\nkind: Stack\nmetadata:\n  name: k\nimports:\n  files:\n    - url: " + src1 + "\n    - url: " + src2 + "\n"
	stackPath := writeTempBlueprint(t, dir, "stack.yaml", stack)

	m := New(stackPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("Merge() error: %v", err)
	}
	count := 0
	for _, n := range m.ToFunctions().Definitions {
		if n.Name == "inBusinessHours" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("inBusinessHours appears %d times, want exactly 1", count)
	}
}
