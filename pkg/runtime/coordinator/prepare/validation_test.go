package prepare

import (
	"github.com/intentd/inrun/pkg/utils"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func validationObj(spec map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "demo.inrun.dev/v1",
		"kind":       "Website",
		"metadata": map[string]interface{}{
			"name":      "test-site",
			"namespace": "default",
		},
		"spec": spec,
	}
}

func TestRunValidation_NilConfig_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{})
	result := runValidation(obj, nil, nil, "Website")
	if !result.Passed {
		t.Error("nil config must return Passed=true")
	}
	if result.HasViolations() {
		t.Error("nil config must produce no violations")
	}
}

func TestRunValidation_EmptyRules_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{})
	cfg := &types.ValidationConfig{}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Error("empty rules must return Passed=true")
	}
}

func TestRunValidation_Equals_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"env": "production"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.env", Equals: "production", Message: "env must be production"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("expected pass, got violations: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Equals_Fails(t *testing.T) {
	obj := validationObj(map[string]interface{}{"env": "staging"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.env", Equals: "production", Message: "env must be production"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Passed {
		t.Error("expected failure")
	}
	if !result.HasViolations() {
		t.Error("expected at least one violation")
	}
	if result.Violations[0].Field != "spec.env" {
		t.Errorf("violation field: expected spec.env, got %q", result.Violations[0].Field)
	}
}

func TestRunValidation_Exists_FieldPresent_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"image": "nginx:1.25"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Exists: utils.BoolPtr(true), Message: "image is required"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("exists check should pass when field is present: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Exists_FieldAbsent_Fails(t *testing.T) {
	obj := validationObj(map[string]interface{}{})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Exists: utils.BoolPtr(true), Message: "image is required"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Passed {
		t.Error("exists check should fail when field is absent")
	}
}

func TestRunValidation_Prefix_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"image": "myorg/nginx:1.25"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Prefix: "myorg/", Message: "image must come from myorg"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("prefix check should pass: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Prefix_Fails(t *testing.T) {
	obj := validationObj(map[string]interface{}{"image": "public.io/nginx:1.25"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Prefix: "myorg/", Message: "image must come from myorg"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Passed {
		t.Error("prefix check should fail when prefix does not match")
	}
}

func TestRunValidation_Suffix_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"image": "myorg/nginx:1.25"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Suffix: ":1.25", Message: "must pin to 1.25"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("suffix check should pass: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Contains_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"image": "myorg/nginx:1.25"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Contains: "nginx", Message: "must use nginx"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("contains check should pass: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Min_AtMinValue_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"replicas": "3"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.replicas", Min: "3", Message: "replicas must be >= 3"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("min check at exact value should pass: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Min_BelowMin_Fails(t *testing.T) {
	obj := validationObj(map[string]interface{}{"replicas": "1"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.replicas", Min: "3", Message: "replicas must be >= 3"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Passed {
		t.Error("min check should fail when value is below minimum")
	}
}

func TestRunValidation_Max_AtMaxValue_Passes(t *testing.T) {
	obj := validationObj(map[string]interface{}{"replicas": "10"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.replicas", Max: "10", Message: "replicas must be <= 10"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if !result.Passed {
		t.Errorf("max check at exact value should pass: %s", result.ViolationSummary())
	}
}

func TestRunValidation_Max_AboveMax_Fails(t *testing.T) {
	obj := validationObj(map[string]interface{}{"replicas": "20"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.replicas", Max: "10", Message: "replicas must be <= 10"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Passed {
		t.Error("max check should fail when value exceeds maximum")
	}
}

func TestRunValidation_Min_NonNumericValue_IsSkipped(t *testing.T) {
	obj := validationObj(map[string]interface{}{"replicas": "not-a-number"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.replicas", Min: "3", Message: "must be >= 3"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	_ = result
}

func TestRunValidation_MultipleViolations_AllCollected(t *testing.T) {
	obj := validationObj(map[string]interface{}{"env": "staging"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.env", Equals: "production", Message: "env must be production"},
			{Field: "spec.image", Exists: utils.BoolPtr(true), Message: "image required"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Passed {
		t.Error("expected failure with multiple violations")
	}
	if len(result.Violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(result.Violations))
	}
}

func TestValidationResult_Error_NilWhenPassed(t *testing.T) {
	obj := validationObj(map[string]interface{}{"env": "production"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.env", Equals: "production"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.Error() != nil {
		t.Errorf("expected nil error for passing result, got: %v", result.Error())
	}
}

func TestValidationResult_Error_MessageIncludesViolation(t *testing.T) {
	obj := validationObj(map[string]interface{}{"env": "staging"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.env", Equals: "production", Message: "must be production"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	err := result.Error()
	if err == nil {
		t.Fatal("expected error for failed validation")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestValidationResult_ViolationSummary_EmptyWhenPassed(t *testing.T) {
	obj := validationObj(map[string]interface{}{"env": "production"})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.env", Equals: "production"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.ViolationSummary() != "" {
		t.Errorf("expected empty summary for passing result, got %q", result.ViolationSummary())
	}
}

func TestValidationResult_ViolationSummary_NonEmpty(t *testing.T) {
	obj := validationObj(map[string]interface{}{})
	cfg := &types.ValidationConfig{
		Rules: []types.ValidationRule{
			{Field: "spec.image", Exists: utils.BoolPtr(true), Message: "image required"},
		},
	}
	result := runValidation(obj, nil, cfg, "Website")
	if result.ViolationSummary() == "" {
		t.Error("expected non-empty violation summary for failing result")
	}
}
