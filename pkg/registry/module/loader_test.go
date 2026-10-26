package module_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/types"
)

func TestLoad_ValidFile(t *testing.T) {
	m, err := module.Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Metadata.Name != "test-db" {
		t.Errorf("name = %q, want %q", m.Metadata.Name, "test-db")
	}
	if m.Kind != "Module" {
		t.Errorf("kind = %q, want Module", m.Kind)
	}
	if len(m.Inputs) != 2 {
		t.Errorf("inputs len = %d, want 2", len(m.Inputs))
	}
}

func TestLoad_WrongKind(t *testing.T) {
	_, err := module.Load("testdata/wrong_kind.yaml")
	if err == nil {
		t.Fatal("expected error for wrong kind, got nil")
	}
}

func TestLoad_NoName(t *testing.T) {
	_, err := module.Load("testdata/no_name.yaml")
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := module.Load("testdata/does_not_exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadImport_FilePath(t *testing.T) {
	imp := &types.ModuleImport{Module: "testdata/valid.yaml"}
	m, err := module.LoadImport(imp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Metadata.Name != "test-db" {
		t.Errorf("name = %q, want test-db", m.Metadata.Name)
	}
}

func TestLoadImport_RelativePath(t *testing.T) {
	imp := &types.ModuleImport{Module: "./testdata/simple.yaml"}
	m, err := module.LoadImport(imp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Metadata.Name != "simple" {
		t.Errorf("name = %q, want simple", m.Metadata.Name)
	}
}
