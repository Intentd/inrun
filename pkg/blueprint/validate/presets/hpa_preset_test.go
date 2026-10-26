package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
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

func TestValidateHPABehaviorPresets_NoCRDs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_NoPreset(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{Name: "hpa"})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_ValidPreset_Web(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{Preset: "web"},
	})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_ValidPreset_API(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Preset: "api"},
	})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_ValidPreset_LatencySensitive(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Preset: "latency-sensitive"},
	})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_ValidPreset_Batch(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Preset: "batch"},
	})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_ValidPreset_CostOptimized(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Preset: "cost-optimized"},
	})
	assert.NoError(t, e.validateHPABehaviorPresets())
}

func TestValidateHPABehaviorPresets_UnknownPreset(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{Preset: "aggressive"},
	})
	err := e.validateHPABehaviorPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown behavior.preset")
	assert.Contains(t, err.Error(), "aggressive")
	assert.Contains(t, err.Error(), "web")
}

func TestValidateHPABehaviorPresets_MixedWithScaleUp(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Preset:  "web",
			ScaleUp: &types.HPAScalingRules{StabilizationWindowSeconds: 30},
		},
	})
	err := e.validateHPABehaviorPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scaleUp/scaleDown")
}

func TestValidateHPABehaviorPresets_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithHPA("app", types.HPATemplateSource{
		Behavior: &types.HPABehavior{Preset: "{{ .Spec.HPA }}"},
	})
	assert.NoError(t, e.validateHPABehaviorPresets())
}
