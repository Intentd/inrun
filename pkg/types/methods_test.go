// Tests for CRDEntry methods declared in methods.go.
// All methods follow a nil-pointer-safe / default-value pattern.
package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func boolp(v bool) *bool { return &v }

func emptyCRD() types.CRDEntry {
	return types.CRDEntry{OperatorBox: &types.OperatorBoxConfig{}}
}

// ── SetQueueDepth ──────────────────────────────────────────────────────────

func TestSetQueueDepth_UsesDefault(t *testing.T) {
	c := emptyCRD()
	assert.Equal(t, 10, c.SetQueueDepth(10))
}

func TestSetQueueDepth_UsesPerCRDValue(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{Queue: types.Queue{MaxDepth: 25}}
	assert.Equal(t, 25, c.SetQueueDepth(10))
}

// ── SetWorkers ────────────────────────────────────────────────────────────────

func TestSetWorkers_UsesDefault(t *testing.T) {
	c := emptyCRD()
	assert.Equal(t, 4, c.SetWorkers(4))
}

func TestSetWorkers_UsesPerCRDValue(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{Workers: 8}
	assert.Equal(t, 8, c.SetWorkers(4))
}

// ── IsDynamic ─────────────────────────────────────────────────────────────────

func TestIsDynamic_ExplicitDynamic(t *testing.T) {
	c := emptyCRD()
	c.Mode = types.CRDModeDynamic
	assert.True(t, c.IsDynamic())
}

func TestIsDynamic_ExplicitTyped(t *testing.T) {
	c := emptyCRD()
	c.Mode = types.CRDModeTyped
	assert.False(t, c.IsDynamic())
}

func TestIsDynamic_NoLocationDefaultsDynamic(t *testing.T) {
	c := emptyCRD()
	// No mode, no location — defaults to dynamic
	assert.True(t, c.IsDynamic())
}

func TestIsDynamic_WithLocationDefaultsTyped(t *testing.T) {
	c := emptyCRD()
	c.APITypes.Location = "pkg/crds"
	assert.False(t, c.IsDynamic())
}

// ── IsEnabled ─────────────────────────────────────────────────────────────────

func TestIsEnabled_NilDefaultsTrue(t *testing.T) {
	c := emptyCRD()
	assert.True(t, c.IsEnabled())
}

func TestIsEnabled_ExplicitTrue(t *testing.T) {
	c := emptyCRD()
	c.Enabled = boolp(true)
	assert.True(t, c.IsEnabled())
}

func TestIsEnabled_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	c.Enabled = boolp(false)
	assert.False(t, c.IsEnabled())
}

// ── IsNamespaced ──────────────────────────────────────────────────────────────

func TestIsNamespaced_NilDefaultsTrue(t *testing.T) {
	c := emptyCRD()
	assert.True(t, c.IsNamespaced())
}

func TestIsNamespaced_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	c.Namespaced = boolp(false)
	assert.False(t, c.IsNamespaced())
}

// ── DefaultReconcile ──────────────────────────────────────────────────────────

func TestDefaultReconcile_NilDefaultsTrue(t *testing.T) {
	c := emptyCRD()
	assert.True(t, c.DefaultReconcile())
}

func TestDefaultReconcile_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{Default: boolp(false)}
	assert.False(t, c.DefaultReconcile())
}

// ── DefaultQueue ──────────────────────────────────────────────────────────────

func TestDefaultQueue_NilDefaultsFalse(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.SharedQueue())
}

func TestDefaultQueue_ExplicitTrue(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{Queue: types.Queue{Shared: boolp(true)}}
	assert.True(t, c.SharedQueue())
}

// ── IsHealthEnabled / IsInfoEnabled / IsEnabledAllEndpoints ──────────────────

func TestIsHealthEnabled_NilDefaultsTrue(t *testing.T) {
	c := emptyCRD()
	assert.True(t, c.IsHealthEnabled())
}

func TestIsHealthEnabled_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	c.Endpoints.Health = boolp(false)
	assert.False(t, c.IsHealthEnabled())
}

func TestIsInfoEnabled_NilDefaultsTrue(t *testing.T) {
	c := emptyCRD()
	assert.True(t, c.IsInfoEnabled())
}

func TestIsInfoEnabled_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	c.Endpoints.Info = boolp(false)
	assert.False(t, c.IsInfoEnabled())
}

func TestIsEnabledAllEndpoints_NilDefaultsTrue(t *testing.T) {
	c := emptyCRD()
	assert.True(t, c.IsEnabledAllEndpoints())
}

func TestIsEnabledAllEndpoints_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	c.Endpoints.Enabled = boolp(false)
	assert.False(t, c.IsEnabledAllEndpoints())
}

// ── GVK / GVR strings ────────────────────────────────────────────────────────

func TestGVKString(t *testing.T) {
	c := emptyCRD()
	c.GroupVersionKind = schema.GroupVersionKind{Group: "demo.io", Version: "v1", Kind: "Website"}
	assert.Equal(t, "demo.io/v1, Kind=Website", c.GVKString())
}

func TestGVRString(t *testing.T) {
	c := emptyCRD()
	c.GroupVersionResource = schema.GroupVersionResource{Group: "demo.io", Version: "v1", Resource: "websites"}
	assert.Equal(t, "demo.io/v1, Resource=websites", c.GVRString())
}

// ── HasValidationOrMutationRules ─────────────────────────────────────────────

func TestHasValidationOrMutationRules_Empty(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasValidationOrMutationRules())
}

func TestHasValidationRules_WithRules(t *testing.T) {
	c := emptyCRD()
	c.Admission = &types.AdmissionConfig{
		Validation: &types.ValidationConfig{
			Rules: []types.ValidationRule{{Field: "spec.image"}},
		},
	}
	assert.True(t, c.HasValidationRules())
	assert.True(t, c.HasValidationOrMutationRules())
	assert.False(t, c.HasMutationRules())
}

func TestHasMutationRules_WithRules(t *testing.T) {
	c := emptyCRD()
	c.Admission = &types.AdmissionConfig{
		Mutation: &types.MutationConfig{
			Rules: []types.MutationRule{{Field: "spec.replicas", Default: "1"}},
		},
	}
	assert.True(t, c.HasMutationRules())
	assert.True(t, c.HasValidationOrMutationRules())
	assert.False(t, c.HasValidationRules())
}

func TestHasMutationRules_NilMutation(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasMutationRules())
}

func TestHasValidationRules_NilValidation(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasValidationRules())
}

// ── HasOnCreate / HasOnReconcile / HasOnDelete / HasAnyHooks ─────────────────

func TestHasOnCreate_False(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasOnCreate())
}

func TestHasOnCreate_True(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{OnCreate: &types.HookTemplates{}}
	assert.True(t, c.HasOnCreate())
}

func TestHasOnReconcile_True(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{OnReconcile: &types.HookTemplates{}}
	assert.True(t, c.HasOnReconcile())
}

func TestHasOnDelete_True(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{OnDelete: &types.HookTemplates{}}
	assert.True(t, c.HasOnDelete())
}

func TestHasAnyHooks_None(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasAnyHookTemplates())
}

func TestHasAnyHooks_OnCreateOnly(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{OnCreate: &types.HookTemplates{}}
	assert.True(t, c.HasAnyHookTemplates())
}

// ── HasTemplates ─────────────────────────────────────────────────────────────

func TestHasTemplates_None(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasTemplates())
}

func TestHasTemplates_OnReconcile(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{OnReconcile: &types.HookTemplates{}}
	assert.True(t, c.HasTemplates())
}

// ── IsNotificationEnabled ────────────────────────────────────────────────────

// ── ValidateMetricField ───────────────────────────────────────────────────────

func TestValidateMetricField_KnownFields(t *testing.T) {
	c := emptyCRD()
	known := []string{
		"metrics.workersBusyPercent",
		"metrics.workersIdlePercent",
		"metrics.queueDepth",
		"metrics.reconcileDurationP95Ms",
		"metrics.errorRatePercent",
	}
	for _, f := range known {
		assert.NoError(t, c.ValidateMetricField(f), "field %q should be valid", f)
	}
}

func TestValidateMetricField_Unknown(t *testing.T) {
	c := emptyCRD()
	err := c.ValidateMetricField("metrics.unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown autoscale metric field")
}

// ── InvolvedInConversion ─────────────────────────────────────────────────────

func TestInvolvedInConversion_Nil(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.InvolvedInConversion())
}

func TestInvolvedInConversion_WithPaths(t *testing.T) {
	c := emptyCRD()
	c.Admission = &types.AdmissionConfig{Conversion: &types.CRDConversion{
		Paths: []types.ConversionPath{{From: "v1", To: "v2"}},
	}}
	assert.True(t, c.InvolvedInConversion())
}

func TestInvolvedInConversion_Participant(t *testing.T) {
	c := emptyCRD()
	c.Admission = &types.AdmissionConfig{Conversion: &types.CRDConversion{Participant: true}}
	assert.True(t, c.InvolvedInConversion())
}

// ── ShouldManageCRD ────────────────────────────────────────────────────────

func TestShouldManageCRD_NoConversion(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.ShouldManageCRD())
}

func TestShouldManageCRD_DefaultsOnWithPaths(t *testing.T) {
	c := emptyCRD()
	c.Admission = &types.AdmissionConfig{Conversion: &types.CRDConversion{
		StorageVersion: "v1",
		Paths:          []types.ConversionPath{{From: "v1alpha1", To: "v1"}},
	}}
	assert.True(t, c.ShouldManageCRD())
}

func TestShouldManageCRD_DefaultsOffForParticipant(t *testing.T) {
	c := emptyCRD()
	c.Admission = &types.AdmissionConfig{Conversion: &types.CRDConversion{Participant: true}}
	assert.False(t, c.ShouldManageCRD())
}

func TestShouldManageCRD_ExplicitFalse(t *testing.T) {
	c := emptyCRD()
	off := false
	c.Admission = &types.AdmissionConfig{Conversion: &types.CRDConversion{
		Paths:     []types.ConversionPath{{From: "v1alpha1", To: "v1"}},
		ManageCRD: &off,
	}}
	assert.False(t, c.ShouldManageCRD())
}

// ── HasNamespaceRules ─────────────────────────────────────────────────────────

func TestHasNamespaceRules_Empty(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.HasNamespaceRules())
}

func TestHasNamespaceRules_WithAllowed(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{AllowedNamespaces: types.AllowedNamespaces{"apps"}}
	assert.True(t, c.HasNamespaceRules())
}

func TestHasNamespaceRules_WithRestricted(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{RestrictedNamespaces: types.RestrictedNamespaces{"kube-system"}}
	assert.True(t, c.HasNamespaceRules())
}

// ── AllowedNamespacesOnly / RestrictedNamespacesOnly ─────────────────────────

func TestAllowedNamespacesOnly_OnlyAllowed(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{AllowedNamespaces: types.AllowedNamespaces{"apps"}}
	assert.True(t, c.AllowedNamespacesOnly())
	assert.False(t, c.RestrictedNamespacesOnly())
}

func TestRestrictedNamespacesOnly_OnlyRestricted(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{RestrictedNamespaces: types.RestrictedNamespaces{"kube-system"}}
	assert.True(t, c.RestrictedNamespacesOnly())
	assert.False(t, c.AllowedNamespacesOnly())
}

func TestAllowedAndRestrictedBoth_NeitherOnly(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{
		AllowedNamespaces:    types.AllowedNamespaces{"apps"},
		RestrictedNamespaces: types.RestrictedNamespaces{"kube-system"},
	}
	assert.False(t, c.AllowedNamespacesOnly())
	assert.False(t, c.RestrictedNamespacesOnly())
}

// ── AutoscaleEnabled / HasAutoscaleProfile ───────────────────────────────────

func TestAutoscaleEnabled_Nil(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.AutoscaleEnabled())
}

func TestAutoscaleEnabled_Set(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{Autoscale: &types.AutoscaleSpec{}}
	assert.True(t, c.AutoscaleEnabled())
}

func TestHasAutoscaleProfile_NoProfile(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{Autoscale: &types.AutoscaleSpec{}}
	assert.False(t, c.HasAutoscaleProfile())
}

func TestHasAutoscaleProfile_WithProfile(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Runtime = &types.RuntimeConfig{Autoscale: &types.AutoscaleSpec{Profile: "burst"}}
	assert.True(t, c.HasAutoscaleProfile())
	assert.Equal(t, "burst", c.AutoScaleProfile())
}

// ── WithHooksDecl / WithConstructorDecl ───────────────────────────────────────

func TestWithHooksDecl_Nil(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.WithHooksDecl())
}

func TestWithHooksDecl_WithLocation(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{Hooks: &types.HookDeclaration{Location: "hooks/"}}
	assert.True(t, c.WithHooksDecl())
}

func TestWithConstructorDecl_Nil(t *testing.T) {
	c := emptyCRD()
	assert.False(t, c.WithConstructorDecl())
}

func TestWithConstructorDecl_WithLocation(t *testing.T) {
	c := emptyCRD()
	c.OperatorBox.Reconcile = &types.ReconcileConfig{ConstructorDecl: &types.ConstructorDeclaration{Location: "cmd/"}}
	assert.True(t, c.WithConstructorDecl())
}
