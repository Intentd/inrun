package validate

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithValidationRule(crdName string, rules ...types.ValidationRule) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {
			Name:      crdName,
			Admission: &types.AdmissionConfig{Validation: &types.ValidationConfig{Rules: rules}},
		},
	})
}

func catalogWithMutationRule(crdName string, rules ...types.MutationRule) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {
			Name:      crdName,
			Admission: &types.AdmissionConfig{Mutation: &types.MutationConfig{Rules: rules}},
		},
	})
}

func TestValidateAdmissionOperators_NoCRDs(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateAdmissionOperators())
}

func TestValidateAdmissionOperators_NoOperatorSet(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.image", Prefix: "myorg/", Message: "must be from myorg",
	})
	assert.NoError(t, e.validateAdmissionOperators())
}

func TestValidateAdmissionOperators_KnownOperator(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Operator: types.ConditionLte, Value: "10", Message: "too many replicas",
	})
	assert.NoError(t, e.validateAdmissionOperators())
}

func TestValidateAdmissionOperators_UnknownOperator(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Operator: "lte2", Value: "10", Message: "too many replicas",
	})
	err := e.validateAdmissionOperators()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lte2")
	assert.Contains(t, err.Error(), "app")
	assert.Contains(t, err.Error(), "spec.replicas")
}

func TestValidateAdmissionOperators_UnknownOperatorInWhen(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Prefix: "x", Message: "msg",
		When: []types.Condition{{Field: "spec.tier", Operator: "greaterOrEqual"}},
	})
	err := e.validateAdmissionOperators()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "greaterOrEqual")
}

func TestValidateAdmissionOperators_UnknownOperatorInOr(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Prefix: "x", Message: "msg",
		Or: []types.Condition{{Field: "spec.tier", Operator: "notARealOp"}},
	})
	err := e.validateAdmissionOperators()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notARealOp")
}

func TestValidateAdmissionOperators_MutationRuleUnknownOperatorInWhen(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Field:   "spec.tier",
		Default: "standard",
		When:    []types.Condition{{Field: "spec.env", Operator: "isEqualTo"}},
	})
	err := e.validateAdmissionOperators()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "isEqualTo")
}

func TestValidateAdmissionOperators_MutationRuleKnownOperator(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Field:   "spec.tier",
		Default: "standard",
		When:    []types.Condition{{Field: "spec.env", Operator: types.ConditionGte, Value: "1"}},
	})
	assert.NoError(t, e.validateAdmissionOperators())
}

func catalogWithServeValidationRule(crdName string, srv *types.ServeConfig, rules ...types.ValidationRule) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {
			Name:      crdName,
			Serve:     srv,
			Admission: &types.AdmissionConfig{Validation: &types.ValidationConfig{Rules: rules}},
		},
	})
}

func TestValidateValidationRuleLinks_NoLink(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.image", Prefix: "myorg/", Message: "must be from myorg",
	})
	assert.NoError(t, e.validateValidationRuleLinks())
}

func TestValidateValidationRuleLinks_MatchesAdditionalLabelField(t *testing.T) {
	serve := &types.ServeConfig{
		Enabled: true,
		Labels:  map[string]types.ServeFieldConfig{"team": {Label: "Team"}},
	}
	e := catalogWithServeValidationRule("app", serve, types.ValidationRule{
		Field: `{{ isDNS1123Subdomain team }}`, Link: "team", Equals: "true", Message: "must be a valid subdomain",
	})
	assert.NoError(t, e.validateValidationRuleLinks())
}

func TestValidateValidationRuleLinks_MatchesAdditionalAnnotationField(t *testing.T) {
	serve := &types.ServeConfig{
		Enabled:     true,
		Annotations: map[string]types.ServeFieldConfig{"platform.myorg.io/jira-ticket": {Label: "Jira Ticket"}},
	}
	e := catalogWithServeValidationRule("app", serve, types.ValidationRule{
		Field: `{{ getAnnotation . "platform.myorg.io/jira-ticket" }}`, Link: "platform.myorg.io/jira-ticket", Message: "must be set",
	})
	assert.NoError(t, e.validateValidationRuleLinks())
}

func TestValidateValidationRuleLinks_SpecFieldWithWrappingExpression(t *testing.T) {
	// link: pointing at a spec field is valid when Field wraps it in
	// something other than the plain "spec.<name>" path — e.g. a format
	// check built on a notes: function.
	serve := &types.ServeConfig{
		Enabled: true, Fields: map[string]types.ServeFieldConfig{
			"repoURL": {Label: "Repository URL"},
		}}
	e := catalogWithServeValidationRule("app", serve, types.ValidationRule{
		Field: `{{ isValidGitRepository .spec.repoURL }}`, Link: "repoURL", Equals: "true", Message: "must be a valid git repository",
	})
	assert.NoError(t, e.validateValidationRuleLinks())
}

func TestValidateValidationRuleLinks_RedundantSpecFieldLink(t *testing.T) {
	serve := &types.ServeConfig{
		Enabled: true,
		Fields: map[string]types.ServeFieldConfig{
			"team": {Label: "Team"},
		}}
	e := catalogWithServeValidationRule("app", serve, types.ValidationRule{
		Field: "spec.team", Link: "team", Operator: types.ConditionExists, Message: "team is required",
	})
	err := e.validateValidationRuleLinks()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redundant")
	assert.Contains(t, err.Error(), "team")
}

func TestValidateValidationRuleLinks_UnknownLink(t *testing.T) {
	serve := &types.ServeConfig{
		Enabled: true,
		Labels:  map[string]types.ServeFieldConfig{"team": {Label: "Team"}},
	}
	e := catalogWithServeValidationRule("app", serve, types.ValidationRule{
		Field: `{{ isDNS1123Subdomain typo }}`, Link: "typo", Equals: "true", Message: "must be valid",
	})
	err := e.validateValidationRuleLinks()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "typo")
	assert.Contains(t, err.Error(), "does not match any serve field")
}

func TestValidateValidationRuleLinks_NoServeConfig(t *testing.T) {
	e := catalogWithServeValidationRule("app", nil, types.ValidationRule{
		Field: `{{ isDNS1123Subdomain team }}`, Link: "team", Equals: "true", Message: "must be valid",
	})
	err := e.validateValidationRuleLinks()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team")
}

// Mutation Rules
func TestValidateMutationRules_FieldDefault(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Field:   "metadata.name",
		Default: "my-app",
	})
	err := e.validateMutationRules()
	assert.NoError(t, err)
}

func TestValidateMutationRules_FieldOverride(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Field:    "metadata.name",
		Override: "my-app",
	})
	err := e.validateMutationRules()
	assert.NoError(t, err)
}

func TestValidateMutationRules_FieldEmpty(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Default:   3,
		ValueType: "int",
	})
	err := e.validateMutationRules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field is required")
}

func TestValidateMutationRules_DefaultAndOverride(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Field:     "spec.deploy.replicas",
		Default:   3,
		Override:  6,
		ValueType: "int",
	})

	err := e.validateMutationRules()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.cat.EnabledCRDs()["app"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for default and override definition")
	}

	assert.NoError(t, err)

	containsWarn := crd.Warnings.Contains("mutation.rules[0] has both default and override defined")
	if !containsWarn {
		t.Fatalf("expected true: got %v - %q", containsWarn, strings.Join(crd.Warnings, ", "))
	}
}

// With Serve Synthesis
func TestValidateMutationRules_ServeFieldEmpty(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Default:   3,
		ValueType: "int",
	})
	err := e.validateMutationRules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field is required")
}

func TestValidateMutationRules_ServeFieldOverride(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Fields: map[string]types.ServeFieldConfig{
			"image": {
				Label:    "Image",
				Override: "my-app/v1",
			},
		},
	})
	err := e.validateMutationRules()
	assert.NoError(t, err)
}

func TestValidateMutationRules_ServeFieldDefault(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Fields: map[string]types.ServeFieldConfig{
			"image": {
				Label:   "Image",
				Default: "my-app/v1",
			},
		},
	})

	err := e.validateMutationRules()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'default' is not allowed for serve.fields.image")
}

func TestValidateMutationRules_ServeLabelDefaultAndOverride(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Labels: map[string]types.ServeFieldConfig{
			"image": {
				Label:    "Image",
				Default:  "my-app/v1",
				Override: "my-app/v2",
			},
		},
	})

	err := e.validateMutationRules()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.cat.EnabledCRDs()["myresource"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for default and override definition")
	}

	assert.NoError(t, err)

	containsWarn := crd.Warnings.Contains("serve.labels.image has both default and override defined")
	if !containsWarn {
		t.Fatalf("expected true: got %v - %q", containsWarn, strings.Join(crd.Warnings, ", "))
	}
}

func TestValidateMutationRules_ServeAnnotationDefaultAndOverride(t *testing.T) {
	e := catalogWithServe(&types.ServeConfig{
		Enabled: true,
		Annotations: map[string]types.ServeFieldConfig{
			"image": {
				Label:    "Image",
				Default:  "my-app/v1",
				Override: "my-app/v2",
			},
		},
	})

	err := e.validateMutationRules()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.cat.EnabledCRDs()["myresource"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for default and override definition")
	}

	assert.NoError(t, err)

	containsWarn := crd.Warnings.Contains("serve.annotations.image has both default and override defined")
	if !containsWarn {
		t.Fatalf("expected true: got %v - %q", containsWarn, strings.Join(crd.Warnings, ", "))
	}
}
