package admission

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogWithServe is a checker over one CRD with serve set.
func catalogWithServe(serve *types.ServeConfig) *checker {
	return newCatalogExec(map[string]types.CRDEntry{
		"myresource": {Serve: serve},
	})
}

func catalogWithValidationRule(crdName string, rules ...types.ValidationRule) *checker {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {
			Name:      crdName,
			Admission: &types.AdmissionConfig{Validation: &types.ValidationConfig{Rules: rules}},
		},
	})
}

func catalogWithMutationRule(crdName string, rules ...types.MutationRule) *checker {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {
			Name:      crdName,
			Admission: &types.AdmissionConfig{Mutation: &types.MutationConfig{Rules: rules}},
		},
	})
}

func TestValidateAdmissionComparisons_NoCRDs(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateAdmissionComparisons())
}

func TestValidateAdmissionComparisons_OneComparison(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Max: "10", Message: "too many replicas",
		When: []types.Condition{{Field: "spec.tier", TypeOf: "string"}},
	})
	assert.NoError(t, e.validateAdmissionComparisons())
}

func TestValidateAdmissionComparisons_TwoOnARule(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Min: "1", Max: "10", Message: "out of range",
	})
	err := e.validateAdmissionComparisons()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declares min and max")
	assert.Contains(t, err.Error(), "spec.replicas")
}

func TestValidateAdmissionComparisons_TwoOnACondition(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{
		Field: "spec.replicas", Prefix: "x", Message: "msg",
		Or: []types.Condition{{Field: "spec.tier", Equals: "gold", Exists: ptr.To(true)}},
	})
	err := e.validateAdmissionComparisons()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declares exists and equals")
}

func TestValidateAdmissionComparisons_MutationWhen(t *testing.T) {
	e := catalogWithMutationRule("app", types.MutationRule{
		Field:   "spec.tier",
		Default: "standard",
		When:    []types.Condition{{Field: "spec.env", TypeOf: "list"}},
	})
	err := e.validateAdmissionComparisons()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `typeOf "list" is not a type`)
}

func TestValidateValidationRules_NoComparisonListsThem(t *testing.T) {
	e := catalogWithValidationRule("app", types.ValidationRule{Field: "spec.image", Message: "m"})
	err := e.validateValidationRules()
	require.Error(t, err)
	for _, name := range types.ComparisonNames() {
		assert.Contains(t, err.Error(), name)
	}
}

func catalogWithServeValidationRule(crdName string, srv *types.ServeConfig, rules ...types.ValidationRule) *checker {
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
	// check built on a KEL function.
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
		Field: "spec.team", Link: "team", Exists: ptr.To(true), Message: "team is required",
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
	crd := e.Cat.EnabledCRDs()["app"]
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
	crd := e.Cat.EnabledCRDs()["myresource"]
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
	crd := e.Cat.EnabledCRDs()["myresource"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for default and override definition")
	}

	assert.NoError(t, err)

	containsWarn := crd.Warnings.Contains("serve.annotations.image has both default and override defined")
	if !containsWarn {
		t.Fatalf("expected true: got %v - %q", containsWarn, strings.Join(crd.Warnings, ", "))
	}
}
