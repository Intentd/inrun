package profiles

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blueprintWithHPA(crdName string, hpas ...types.HPATemplateSource) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
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
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_NoProfile(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{Name: "hpa"})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_Web(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{Profile: "web"},
	})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_API(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "api"},
	})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_LatencySensitive(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "latency-sensitive"},
	})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_Batch(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "batch"},
	})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_ValidProfile_CostOptimized(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "cost-optimized"},
	})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}

func TestValidateHPABehaviorProfiles_UnknownProfile(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{Profile: "aggressive"},
	})
	err := e.validateHPABehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown behavior.profile")
	assert.Contains(t, err.Error(), "aggressive")
	assert.Contains(t, err.Error(), "web")
}

func TestValidateHPABehaviorProfiles_MixedWithScaleUp(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Profile: "web",
			ScaleUp: &types.HPAScalingRules{StabilizationWindowSeconds: 30},
		},
	})
	err := e.validateHPABehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scaleUp/scaleDown")
}

func TestValidateHPABehaviorProfiles_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "{{ .Spec.HPA }}"},
	})
	assert.NoError(t, e.validateHPABehaviorProfiles())
}
