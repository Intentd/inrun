package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crdWithDeploymentOnCreate(deps ...types.DeploymentTemplateSource) types.CRDEntry {
	return types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					Deployments: deps,
				},
			},
		},
	}
}

func TestCollectRollingUpdatePresetEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectRollingUpdatePresetEntries())
}

func TestCollectRollingUpdatePresetEntries_NoRollingUpdate(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{Name: "app"})
	assert.Empty(t, c.CollectRollingUpdatePresetEntries())
}

func TestCollectRollingUpdatePresetEntries_RollingUpdateNoPreset(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name:          "app",
		RollingUpdate: &types.RollingUpdateBehavior{},
	})
	assert.Empty(t, c.CollectRollingUpdatePresetEntries())
}

func TestCollectRollingUpdatePresetEntries_PresetReturned(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name:          "app",
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "safe"},
	})
	entries := c.CollectRollingUpdatePresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "app", entries[0].ResourceName)
	assert.Equal(t, "safe", entries[0].Preset)
	assert.False(t, entries[0].Mixed)
}

func TestCollectRollingUpdatePresetEntries_Mixed_MaxSurge(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name: "app",
		RollingUpdate: &types.RollingUpdateBehavior{
			Preset:   "fast",
			MaxSurge: "2",
		},
	})
	entries := c.CollectRollingUpdatePresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectRollingUpdatePresetEntries_Mixed_MaxUnavailable(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		Name: "app",
		RollingUpdate: &types.RollingUpdateBehavior{
			Preset:         "blue-green",
			MaxUnavailable: "0",
		},
	})
	entries := c.CollectRollingUpdatePresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectRollingUpdatePresetEntries_TemplateExpr(t *testing.T) {
	c := crdWithDeploymentOnCreate(types.DeploymentTemplateSource{
		RollingUpdate: &types.RollingUpdateBehavior{Preset: "{{ .Spec.DeployPreset }}"},
	})
	entries := c.CollectRollingUpdatePresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.DeployPreset }}", entries[0].Preset)
}

func TestCollectRollingUpdatePresetEntries_StatefulSet(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					StatefulSets: []types.StatefulSetTemplateSource{
						{Name: "db", RollingUpdate: &types.RollingUpdateBehavior{Preset: "safe"}},
					},
				},
			},
		},
	}
	entries := c.CollectRollingUpdatePresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "safe", entries[0].Preset)
	assert.Equal(t, "db", entries[0].ResourceName)
}

func TestCollectRollingUpdatePresetEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					Deployments: []types.DeploymentTemplateSource{
						{Name: "app", RollingUpdate: &types.RollingUpdateBehavior{Preset: "fast"}},
					},
				},
			},
		},
	}
	entries := c.CollectRollingUpdatePresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "fast", entries[0].Preset)
}
