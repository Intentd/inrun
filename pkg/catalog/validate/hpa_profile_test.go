package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithHPA(crdName string, hpas ...types.HPATemplateSource) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		crdName: {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						HorizontalPodAutoscalers: hpas,
					},
				},
			},
		},
	})
}

func TestValidateHPABehaviorProfiles_NoCRDs(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_NoProfile(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{Name: "hpa"})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_Web(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{Profile: "web"},
	})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_API(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "api"},
	})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_LatencySensitive(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "latency-sensitive"},
	})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_Batch(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "batch"},
	})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_CostOptimized(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "cost-optimized"},
	})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_UnknownProfile(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{Profile: "aggressive"},
	})
	err := k.validateHPABehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown behavior.profile")
	assert.Contains(t, err.Error(), "aggressive")
	assert.Contains(t, err.Error(), "web")
}

func TestValidateHPABehaviorProfiles_MixedWithScaleUp(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Profile: "web",
			ScaleUp: &types.HPAScalingRules{StabilizationWindowSeconds: 30},
		},
	})
	err := k.validateHPABehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scaleUp/scaleDown")
}

func TestValidateHPABehaviorProfiles_TemplateExprSkipped(t *testing.T) {
	k := catalogWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "{{ .Spec.HPA }}"},
	})
	assert.NoError(t, k.validateHPABehaviorProfiles())
}
