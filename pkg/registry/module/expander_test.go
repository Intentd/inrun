package module_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/types"
)

func loadTestModule(t *testing.T, path string) *types.Module {
	t.Helper()
	m, err := module.Load(path)
	if err != nil {
		t.Fatalf("loading module %s: %v", path, err)
	}
	return m
}

func TestExpand_RequiredInputProvided(t *testing.T) {
	m := loadTestModule(t, "testdata/simple.yaml")

	expanded, err := module.Expand(m, map[string]string{
		"image": "nginx",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expanded.OnReconcile == nil {
		t.Fatal("expected OnReconcile to be non-nil")
	}
	if len(expanded.OnReconcile.Deployments) != 1 {
		t.Fatalf("deployments len = %d, want 1", len(expanded.OnReconcile.Deployments))
	}
}

func TestExpand_DefaultFilled(t *testing.T) {
	m := loadTestModule(t, "testdata/simple.yaml")

	expanded, err := module.Expand(m, map[string]string{
		"image": "nginx",
		// "tag" not provided — should use default "latest"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expanded.OnReconcile == nil {
		t.Fatal("expected OnReconcile to be non-nil")
	}
	if len(expanded.OnReconcile.Deployments) != 1 {
		t.Fatalf("deployments len = %d, want 1", len(expanded.OnReconcile.Deployments))
	}
}

func TestExpand_MissingRequiredInput(t *testing.T) {
	m := loadTestModule(t, "testdata/simple.yaml")

	_, err := module.Expand(m, map[string]string{
		// "image" is required but not provided
	})
	if err == nil {
		t.Fatal("expected error for missing required input, got nil")
	}
}

func TestExpand_UnknownInput(t *testing.T) {
	m := loadTestModule(t, "testdata/simple.yaml")

	_, err := module.Expand(m, map[string]string{
		"image":   "nginx",
		"unknown": "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown input, got nil")
	}
}

func TestValidateModuleTemplates_AllDeclared(t *testing.T) {
	m := loadTestModule(t, "testdata/simple.yaml")
	errs := module.ValidateModuleTemplates(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestMergeFrom_Appends(t *testing.T) {
	dst := &types.HookTemplates{}
	src := &types.HookTemplates{
		Deployments: []types.DeploymentTemplateSource{{Name: "app"}},
		Services:    []types.ServiceTemplateSource{{Name: "svc"}},
		External:    []types.ExternalCallSpec{{Name: "health"}},
	}

	dst.MergeFrom(src)

	if len(dst.Deployments) != 1 {
		t.Errorf("deployments = %d, want 1", len(dst.Deployments))
	}
	if len(dst.Services) != 1 {
		t.Errorf("services = %d, want 1", len(dst.Services))
	}
	if len(dst.External) != 1 {
		t.Errorf("external = %d, want 1", len(dst.External))
	}
}

func TestMergeFrom_NilSafe(t *testing.T) {
	dst := &types.HookTemplates{}
	// Should not panic with nil src
	dst.MergeFrom(nil)
	// Should not panic with nil receiver
	var nilDst *types.HookTemplates
	nilDst.MergeFrom(dst)
}

// TestExpand_StaticConditionDropsResource verifies that a resource whose
// when: condition references only inputs (static) is dropped from the
// expanded output when that condition fails at expansion time.
func TestExpand_StaticConditionDropsResource(t *testing.T) {
	m := loadTestModule(t, "testdata/conditional.yaml")

	// enableUI: "false" — the UI deployment should be dropped
	expanded, err := module.Expand(m, map[string]string{
		"image":    "nginx",
		"enableUI": "false",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expanded.OnReconcile == nil {
		t.Fatal("expected OnReconcile to be non-nil")
	}
	for _, d := range expanded.OnReconcile.Deployments {
		if d.Name == "{{ .metadata.name }}-ui" {
			t.Error("UI deployment should have been dropped (enableUI=false) but was present")
		}
	}
}

// TestExpand_StaticConditionKeepsResource verifies that a resource whose
// static condition passes is kept and has its static conditions stripped.
func TestExpand_StaticConditionKeepsResource(t *testing.T) {
	m := loadTestModule(t, "testdata/conditional.yaml")

	// enableUI: "true" — the UI deployment should be present with no conditions
	expanded, err := module.Expand(m, map[string]string{
		"image":    "nginx",
		"enableUI": "true",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found bool
	for _, d := range expanded.OnReconcile.Deployments {
		if d.Name == "{{ .metadata.name }}-ui" {
			found = true
			if len(d.When) != 0 {
				t.Errorf("static conditions should be stripped from kept resource, got %d", len(d.When))
			}
		}
	}
	if !found {
		t.Error("UI deployment should be present (enableUI=true) but was not found")
	}
}

// TestExpand_RuntimeConditionPreserved verifies that a resource whose when:
// condition references a runtime field ({{ .spec.* }}) is kept and has its
// condition preserved for the reconciler — not evaluated at expansion time.
func TestExpand_RuntimeConditionPreserved(t *testing.T) {
	m := loadTestModule(t, "testdata/conditional.yaml")

	expanded, err := module.Expand(m, map[string]string{
		"image":    "nginx",
		"enableUI": "false",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found bool
	for _, d := range expanded.OnReconcile.Deployments {
		if d.Name == "{{ .metadata.name }}" {
			found = true
			if len(d.When) == 0 {
				t.Error("runtime condition should be preserved on the resource")
			}
			if d.When[0].Field != "{{ .spec.tier }}" {
				t.Errorf("runtime condition field = %q, want %q", d.When[0].Field, "{{ .spec.tier }}")
			}
		}
	}
	if !found {
		t.Error("main deployment should be present regardless of enableUI")
	}
}

// TestExpand_ExternalCallsMerged verifies that external: blocks declared in
// module resources are included in the expanded OnReconcile output.
func TestExpand_ExternalCallsMerged(t *testing.T) {
	m := loadTestModule(t, "testdata/conditional.yaml")

	expanded, err := module.Expand(m, map[string]string{
		"image":      "nginx",
		"enableUI":   "false",
		"serviceUrl": "http://localhost:9999",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expanded.OnReconcile == nil {
		t.Fatal("expected OnReconcile to be non-nil")
	}
	if len(expanded.OnReconcile.External) == 0 {
		t.Error("external calls from module resources should be merged into OnReconcile")
	}
	if expanded.OnReconcile.External[0].Name != "healthCheck" {
		t.Errorf("external[0].Name = %q, want %q", expanded.OnReconcile.External[0].Name, "healthCheck")
	}
}
