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

func TestCollectHPAProfileEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectHPAProfileEntries())
}

func TestCollectHPAProfileEntries_NoBehavior(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{Name: "hpa"})
	assert.Empty(t, c.CollectHPAProfileEntries())
}

func TestCollectHPAProfileEntries_BehaviorNoProfile(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name:     "hpa",
		Behavior: &types.HPABehavior{},
	})
	assert.Empty(t, c.CollectHPAProfileEntries())
}

func TestCollectHPAProfileEntries_ProfileReturned(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name:     "my-hpa",
		Behavior: &types.HPABehavior{Profile: "web"},
	})
	entries := c.CollectHPAProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "my-hpa", entries[0].ResourceName)
	assert.Equal(t, "web", entries[0].Profile)
	assert.False(t, entries[0].Mixed)
}

func TestCollectHPAProfileEntries_Mixed_ScaleUp(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Profile: "batch",
			ScaleUp: &types.HPAScalingRules{StabilizationWindowSeconds: 30},
		},
	})
	entries := c.CollectHPAProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectHPAProfileEntries_Mixed_ScaleDown(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Name: "hpa",
		Behavior: &types.HPABehavior{
			Profile:   "cost-optimized",
			ScaleDown: &types.HPAScalingRules{StabilizationWindowSeconds: 300},
		},
	})
	entries := c.CollectHPAProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectHPAProfileEntries_TemplateExpr(t *testing.T) {
	c := crdWithHPAOnCreate(types.HPATemplateSource{
		Behavior: &types.HPABehavior{Profile: "{{ .Spec.ScaleProfile }}"},
	})
	entries := c.CollectHPAProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.ScaleProfile }}", entries[0].Profile)
}

func TestCollectHPAProfileEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					HorizontalPodAutoscalers: []types.HPATemplateSource{
						{Name: "hpa", Behavior: &types.HPABehavior{Profile: "api"}},
					},
				},
			},
		},
	}
	entries := c.CollectHPAProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "api", entries[0].Profile)
}
