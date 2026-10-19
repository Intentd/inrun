package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crdWithHPAOnCreate(hpas ...types.HPATemplateSource) types.CRDEntry {
	return types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					HorizontalPodAutoscalers: hpas,
				},
			},
		},
	}
}

func TestCollectHPAPresetEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectHPAPresetEntries())
}

func TestCollectHPAPresetEntries_NoBehavior(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{Name: "hpa"})
	assert.Empty(t, c.CollectHPAPresetEntries())
}

func TestCollectHPAPresetEntries_BehaviorNoPreset(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{},
	})
	assert.Empty(t, c.CollectHPAPresetEntries())
}

func TestCollectHPAPresetEntries_PresetReturned(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name:     "my-hpa",
		Behavior: &types.HPABehavior{Preset: "web"},
	})
	entries := c.CollectHPAPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "my-hpa", entries[0].ResourceName)
	assert.Equal(t, "web", entries[0].Preset)
	assert.False(t, entries[0].Mixed)
}

func TestCollectHPAPresetEntries_Mixed_ScaleUp(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Preset:  "batch",
			ScaleUp: &types.HPAScalingRules{StabilizationWindowSeconds: 30},
		},
	})
	entries := c.CollectHPAPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectHPAPresetEntries_Mixed_ScaleDown(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Preset:    "cost-optimized",
			ScaleDown: &types.HPAScalingRules{StabilizationWindowSeconds: 300},
		},
	})
	entries := c.CollectHPAPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectHPAPresetEntries_TemplateExpr(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Behavior: &types.HPABehavior{Preset: "{{ .Spec.ScalePreset }}"},
	})
	entries := c.CollectHPAPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.ScalePreset }}", entries[0].Preset)
}

func TestCollectHPAPresetEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					HorizontalPodAutoscalers: []types.HPATemplateSource{
						{Name: "hpa", Behavior: &types.HPABehavior{Preset: "api"}},
					},
				},
			},
		},
	}
	entries := c.CollectHPAPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "api", entries[0].Preset)
}
