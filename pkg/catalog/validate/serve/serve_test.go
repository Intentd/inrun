package serve

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func catalogWithServe(serve *types.ServeConfig) *checker {
	return newCatalogExec(map[string]types.CRDEntry{
		"myresource": {Serve: serve},
	})
}

func TestValidateServeLabelAndAnnotationFields_NilServe(t *testing.T) {
	e := catalogWithServe(nil)
	if err := e.validateServeLabelAndAnnotationFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_NoLabelsAndAnnotations(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{Enabled: true})
	if err := e.validateServeLabelAndAnnotationFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidateServeLabelAndAnnotationFields_TypeOmittedDefaultsToValid guards against a
// regression where an omitted Type (which defaults to "string" per
// ServeFieldConfig's doc comment) was incorrectly rejected as an invalid type.
// Deliberately uses a single field with no collision and no key-syntax issue
// so nothing else can make this pass for the wrong reason.
func TestValidateServeLabelAndAnnotationFields_TypeOmittedDefaultsToValid(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Labels: map[string]types.ServeFieldConfig{
			"tier": {Label: "Tier"}, // Type intentionally omitted
		},
	})
	if err := e.validateServeLabelAndAnnotationFields(); err != nil {
		t.Fatalf("omitted Type should default to valid (string), got error: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_InvalidType(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Labels: map[string]types.ServeFieldConfig{
			"tier": {Label: "Tier", Type: "sting"}, // typo
		},
	})
	err := e.validateServeLabelAndAnnotationFields()
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "sting") {
		t.Errorf("error should mention the bad type, got: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_Valid(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Fields: map[string]types.ServeFieldConfig{
			"image": {Label: "Image"},
		},
		Labels: map[string]types.ServeFieldConfig{
			"tier": {Label: "Tier", Type: "enum", Enum: []string{"free", "pro"}},
		},
		Annotations: map[string]types.ServeFieldConfig{
			"platform.example.io/monitoring": {Label: "Monitoring", Type: "boolean"},
		},
	})
	if err := e.validateServeLabelAndAnnotationFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_InvalidLabelKey(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Labels: map[string]types.ServeFieldConfig{
			"Not Valid Key!": {Label: "Bad"},
		},
	})
	err := e.validateServeLabelAndAnnotationFields()
	if err == nil {
		t.Fatal("expected error for invalid label key")
	}
	if !strings.Contains(err.Error(), "Not Valid Key!") {
		t.Errorf("error should mention the bad key, got: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_InvalidAnnotationKey(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Annotations: map[string]types.ServeFieldConfig{
			"has spaces": {Label: "Bad"},
		},
	})
	if err := e.validateServeLabelAndAnnotationFields(); err == nil {
		t.Fatal("expected error for invalid annotation key")
	}
}

func TestValidateServeLabelAndAnnotationFields_EnumEmpty(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Labels: map[string]types.ServeFieldConfig{
			"tier": {Label: "Tier", Type: "enum"},
		},
	})
	err := e.validateServeLabelAndAnnotationFields()
	if err == nil {
		t.Fatal("expected error for empty enum")
	}
	if !strings.Contains(err.Error(), "tier") {
		t.Errorf("error should mention the field name, got: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_CollisionWithFields(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Fields: map[string]types.ServeFieldConfig{
			"image": {Label: "Image"},
		},
		Labels: map[string]types.ServeFieldConfig{
			"image": {Label: "Image dup"},
		},
	})
	err := e.validateServeLabelAndAnnotationFields()
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !strings.Contains(err.Error(), "image") {
		t.Errorf("error should mention the colliding key, got: %v", err)
	}
}

func TestValidateServeLabelAndAnnotationFields_CollisionBetweenLabelsAndAnnotations(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Labels: map[string]types.ServeFieldConfig{
			"tier": {Label: "Tier (label)"},
		},
		Annotations: map[string]types.ServeFieldConfig{
			"tier": {Label: "Tier (annotation)"},
		},
	})
	if err := e.validateServeLabelAndAnnotationFields(); err == nil {
		t.Fatal("expected collision error between labels and annotations")
	}
}

func TestValidateServeLabelAndAnnotationFields_MultipleCRDs(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{
		"good": {Serve: &types.ServeConfig{
			Enabled: true,
			Labels:  map[string]types.ServeFieldConfig{"tier": {}},
		}},
		"bad": {Serve: &types.ServeConfig{
			Enabled: true,
			Labels:  map[string]types.ServeFieldConfig{"Not Valid!": {}},
		}},
	})
	if err := e.validateServeLabelAndAnnotationFields(); err == nil {
		t.Fatal("expected error from the bad CRD entry")
	}
}

func TestValidateServeFieldOrder_NoCollision(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Fields: map[string]types.ServeFieldConfig{
			"image":    {Order: 1},
			"replicas": {Order: 2},
		},
	})
	if err := e.validateServeFieldOrder(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServeFieldOrder_UnsetNeverCollides(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Fields: map[string]types.ServeFieldConfig{
			"image":    {},
			"replicas": {},
		},
	})
	if err := e.validateServeFieldOrder(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServeFieldOrder_Collision(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Fields: map[string]types.ServeFieldConfig{
			"image": {Order: 3},
		},
		Labels: map[string]types.ServeFieldConfig{
			"team": {Order: 3},
		},
	})
	err := e.validateServeFieldOrder()
	if err == nil {
		t.Fatal("expected a collision error")
	}
	if !strings.Contains(err.Error(), "image") || !strings.Contains(err.Error(), "team") {
		t.Errorf("error = %q, want it to name both colliding fields", err.Error())
	}
}
