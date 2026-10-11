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

func TestCollectPDBProfileEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectPDBProfileEntries())
}

func TestCollectPDBProfileEntries_NoBehavior(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{Name: "pdb"})
	assert.Empty(t, c.CollectPDBProfileEntries())
}

func TestCollectPDBProfileEntries_BehaviorNoProfile(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name:     "pdb",
		Behavior: &types.PDBBehavior{},
	})
	assert.Empty(t, c.CollectPDBProfileEntries())
}

func TestCollectPDBProfileEntries_ProfileReturned(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name:     "my-pdb",
		Behavior: &types.PDBBehavior{Profile: "zero-downtime"},
	})
	entries := c.CollectPDBProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "my-pdb", entries[0].ResourceName)
	assert.Equal(t, "zero-downtime", entries[0].Profile)
	assert.False(t, entries[0].Mixed)
}

func TestCollectPDBProfileEntries_Mixed_MinAvailable(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Profile:      "rolling",
			MinAvailable: "1",
		},
	})
	entries := c.CollectPDBProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectPDBProfileEntries_Mixed_MaxUnavailable(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Profile:        "relaxed",
			MaxUnavailable: "1",
		},
	})
	entries := c.CollectPDBProfileEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectPDBProfileEntries_TemplateExpr(t *testing.T) {
	c := crdWithPDBOnCreate(types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Profile: "{{ .Spec.PDBProfile }}"},
	})
	entries := c.CollectPDBProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.PDBProfile }}", entries[0].Profile)
}

func TestCollectPDBProfileEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					PodDisruptionBudgets: []types.PDBTemplateSource{
						{Name: "pdb", Behavior: &types.PDBBehavior{Profile: "relaxed"}},
					},
				},
			},
		},
	}
	entries := c.CollectPDBProfileEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
	assert.Equal(t, "relaxed", entries[0].Profile)
}
