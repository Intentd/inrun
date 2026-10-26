// pkg/merger/validate_pattern_test.go
package merger

import (
	"os"
	"path/filepath"
	"testing"
)

func makeBlueprintPatternDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blueprint.yaml"), []byte("kind: Blueprint\n"), 0644); err != nil {
		t.Fatalf("writing blueprint.yaml: %v", err)
	}
	return dir
}

func TestValidatePatternStructure_BlueprintOnly_Valid(t *testing.T) {
	dir := makeBlueprintPatternDir(t)
	if err := validatePatternStructure(dir, "https://example.com/registry", "v1.0"); err != nil {
		t.Errorf("blueprint.yaml alone must pass: %v", err)
	}
}

func TestValidatePatternStructure_ModuleOnly_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "module.yaml"), []byte("kind: Module\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validatePatternStructure(dir, "https://example.com/registry", "v1.0"); err != nil {
		t.Errorf("module.yaml alone must pass: %v", err)
	}
}

func TestValidatePatternStructure_MissingPrimaryFile(t *testing.T) {
	dir := t.TempDir()
	// Only a crd.yaml — no blueprint.yaml or module.yaml
	os.WriteFile(filepath.Join(dir, "crd.yaml"), []byte("content"), 0644)
	if err := validatePatternStructure(dir, "https://example.com/registry", "v1.0"); err == nil {
		t.Error("missing primary file must return error")
	}
}

func TestValidatePatternStructure_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := validatePatternStructure(dir, "https://example.com/registry", "v1.0"); err == nil {
		t.Error("empty dir must return error")
	}
}
