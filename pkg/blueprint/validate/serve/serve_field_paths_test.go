package serve

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func migrateField(path, from string) types.ServeFieldConfig {
	cfg := types.ServeFieldConfig{
		Migrate: &types.ServeFieldMigrate{From: from},
	}
	if path != "" {
		cfg.Path = path
	}
	return cfg
}

func TestValidateServeFieldPaths_DuplicateMigrateFrom_Errors(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"newA": migrateField("dest.a", "oldKey"),
			"newB": migrateField("dest.b", "oldKey"), // same migrate.from
		},
	})
	err := e.validateServeFieldPaths()
	if err == nil {
		t.Fatal("expected error for duplicate migrate.from, got nil")
	}
	if !strings.Contains(err.Error(), "oldKey") {
		t.Errorf("error should name the duplicate path, got: %v", err)
	}
}

func TestValidateServeFieldPaths_DuplicateMigrateFrom_SpecPrefixCollision(t *testing.T) {
	// spec.oldKey and oldKey should collapse to the same effective from-path.
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"newA": migrateField("dest.a", "spec.oldKey"),
			"newB": migrateField("dest.b", "oldKey"),
		},
	})
	err := e.validateServeFieldPaths()
	if err == nil {
		t.Fatal("expected error: spec.oldKey and oldKey resolve to the same from-path")
	}
}

func TestValidateServeFieldPaths_UniqueMigrateFrom_OK(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"newA": migrateField("dest.a", "oldKeyA"),
			"newB": migrateField("dest.b", "oldKeyB"),
		},
	})
	if err := e.validateServeFieldPaths(); err != nil {
		t.Fatalf("unexpected error for unique migrate.from paths: %v", err)
	}
}

func TestValidateServeFieldPaths_SingleMigrateFrom_OK(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"newA":  migrateField("dest.a", "oldKey"),
			"plain": {},
		},
	})
	if err := e.validateServeFieldPaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- overlapping migrate.from paths ---

func TestValidateServeFieldPaths_OverlappingMigrateFrom_ParentChild_Errors(t *testing.T) {
	// "foo" is an ancestor of "foo.bar" — migrating either first corrupts the other.
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"newFoo":    migrateField("dest.a", "foo"),
			"newFooBar": migrateField("dest.b", "foo.bar"),
		},
	})
	err := e.validateServeFieldPaths()
	if err == nil {
		t.Fatal("expected error for overlapping migrate.from paths, got nil")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should name the overlapping paths, got: %v", err)
	}
}

func TestValidateServeFieldPaths_OverlappingMigrateFrom_DeepNesting_Errors(t *testing.T) {
	// "a.b" is an ancestor of "a.b.c.d".
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldX": migrateField("dest.x", "a.b"),
			"fieldY": migrateField("dest.y", "a.b.c.d"),
		},
	})
	if err := e.validateServeFieldPaths(); err == nil {
		t.Fatal("expected error for deep-nested overlapping migrate.from paths")
	}
}

func TestValidateServeFieldPaths_OverlappingMigrateFrom_SiblingPaths_OK(t *testing.T) {
	// "foo.bar" and "foo.baz" share a parent but neither is an ancestor of the other.
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": migrateField("dest.a", "foo.bar"),
			"fieldB": migrateField("dest.b", "foo.baz"),
		},
	})
	if err := e.validateServeFieldPaths(); err != nil {
		t.Fatalf("sibling paths should not be rejected: %v", err)
	}
}

// --- duplicate spec path ---

func TestValidateServeFieldPaths_DuplicateSpecPath_Errors(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: "container.image"},
			"fieldB": {Path: "container.image"}, // same destination
		},
	})
	err := e.validateServeFieldPaths()
	if err == nil {
		t.Fatal("expected error for duplicate spec path, got nil")
	}
	if !strings.Contains(err.Error(), "container.image") {
		t.Errorf("error should name the duplicate path, got: %v", err)
	}
}

func TestValidateServeFieldPaths_UniqueSpecPaths_OK(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: "container.image"},
			"fieldB": {Path: "container.tag"},
		},
	})
	if err := e.validateServeFieldPaths(); err != nil {
		t.Fatalf("unexpected error for unique paths: %v", err)
	}
}

// --- invalid path format ---

func TestValidateServeFieldPaths_EmptySegment_Errors(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: "app..image"}, // double dot
		},
	})
	if err := e.validateServeFieldPaths(); err == nil {
		t.Fatal("expected error for path with empty segment")
	}
}

func TestValidateServeFieldPaths_LeadingDot_Errors(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: ".image"},
		},
	})
	if err := e.validateServeFieldPaths(); err == nil {
		t.Fatal("expected error for path with leading dot")
	}
}

func TestValidateServeFieldPaths_TrailingDot_Errors(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: "image."},
		},
	})
	if err := e.validateServeFieldPaths(); err == nil {
		t.Fatal("expected error for path with trailing dot")
	}
}

// --- nested path warning (not an error) ---

func TestValidateServeFieldPaths_NestedPath_Warns(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: "container.image"},
		},
	})
	if err := e.validateServeFieldPaths(); err != nil {
		t.Fatalf("nested path should warn, not error: %v", err)
	}
	crd := e.Blueprint.EnabledCRDs()["myresource"]
	if !crd.Warnings.HasWarnings() {
		t.Error("expected a warning for nested path, got none")
	}
}

func TestValidateServeFieldPaths_FlatPath_NoWarn(t *testing.T) {
	e := blueprintWithServe(&types.ServeConfig{
		Enabled:   true,
		Namespace: "default",
		Fields: map[string]types.ServeFieldConfig{
			"fieldA": {Path: "image"},
		},
	})
	if err := e.validateServeFieldPaths(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.Blueprint.EnabledCRDs()["myresource"]
	if crd.Warnings.HasWarnings() {
		t.Errorf("flat path should not produce a warning, got: %v", crd.Warnings)
	}
}
