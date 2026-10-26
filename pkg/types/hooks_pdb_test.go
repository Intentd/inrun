package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crdWithPDBOnCreate(pdbs ...types.PDBTemplateSource) types.CRDEntry {
	return types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					PodDisruptionBudgets: pdbs,
				},
			},
		},
	}
}

func TestCollectPDBPresetEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectPDBPresetEntries())
}

func TestCollectPDBPresetEntries_NoBehavior(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{Name: "pdb"})
	assert.Empty(t, c.CollectPDBPresetEntries())
}

func TestCollectPDBPresetEntries_BehaviorNoPreset(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name:     "pdb",
		Behavior: &types.PDBBehavior{},
	})
	assert.Empty(t, c.CollectPDBPresetEntries())
}

func TestCollectPDBPresetEntries_PresetReturned(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name:     "my-pdb",
		Behavior: &types.PDBBehavior{Preset: "zero-downtime"},
	})
	entries := c.CollectPDBPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "my-pdb", entries[0].ResourceName)
	assert.Equal(t, "zero-downtime", entries[0].Preset)
	assert.False(t, entries[0].Mixed)
}

func TestCollectPDBPresetEntries_Mixed_MinAvailable(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Preset:       "rolling",
			MinAvailable: "1",
		},
	})
	entries := c.CollectPDBPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectPDBPresetEntries_Mixed_MaxUnavailable(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Preset:         "relaxed",
			MaxUnavailable: "1",
		},
	})
	entries := c.CollectPDBPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectPDBPresetEntries_TemplateExpr(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Preset: "{{ .Spec.PDBPreset }}"},
	})
	entries := c.CollectPDBPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.PDBPreset }}", entries[0].Preset)
}

func TestCollectPDBPresetEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					PodDisruptionBudgets: []types.PDBTemplateSource{
						{Name: "pdb", Behavior: &types.PDBBehavior{Preset: "relaxed"}},
					},
				},
			},
		},
	}
	entries := c.CollectPDBPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "relaxed", entries[0].Preset)
}
