package catalog_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
)

const validModulePath = "../registry/module/testdata/valid.yaml"
const simpleModulePath = "../registry/module/testdata/simple.yaml"

func TestValidateModule_Valid(t *testing.T) {
	errs := catalog.ValidateModule(validModulePath)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid module, got: %v", errs)
	}
}

func TestValidateModule_NotFound(t *testing.T) {
	errs := catalog.ValidateModule("testdata/does-not-exist.yaml")
	if len(errs) == 0 {
		t.Fatal("expected errors for missing file, got none")
	}
}

func TestValidateModule_WrongKind(t *testing.T) {
	errs := catalog.ValidateModule("../registry/module/testdata/wrong_kind.yaml")
	if len(errs) == 0 {
		t.Fatal("expected errors for wrong kind, got none")
	}
}

func TestValidateModule_NoName(t *testing.T) {
	errs := catalog.ValidateModule("../registry/module/testdata/no_name.yaml")
	if len(errs) == 0 {
		t.Fatal("expected errors for missing name, got none")
	}
}

func TestValidateModule_Simple(t *testing.T) {
	errs := catalog.ValidateModule(simpleModulePath)
	if len(errs) != 0 {
		t.Errorf("expected no errors for simple module, got: %v", errs)
	}
}
