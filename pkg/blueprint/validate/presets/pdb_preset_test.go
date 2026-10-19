package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithPDB(crdName string, pdbs ...types.PDBTemplateSource) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
		crdName: {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						PodDisruptionBudgets: pdbs,
					},
				},
			},
		},
	})
}

func TestValidatePDBBehaviorPresets_NoCRDs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validatePDBBehaviorPresets())
}

func TestValidatePDBBehaviorPresets_NoPreset(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{Name: "pdb"})
	assert.NoError(t, e.validatePDBBehaviorPresets())
}

func TestValidatePDBBehaviorPresets_ValidPreset_ZeroDowntime(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Name:     "pdb",
		Behavior: &types.PDBBehavior{Preset: "zero-downtime"},
	})
	assert.NoError(t, e.validatePDBBehaviorPresets())
}

func TestValidatePDBBehaviorPresets_ValidPreset_Rolling(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Preset: "rolling"},
	})
	assert.NoError(t, e.validatePDBBehaviorPresets())
}

func TestValidatePDBBehaviorPresets_ValidPreset_Relaxed(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Preset: "relaxed"},
	})
	assert.NoError(t, e.validatePDBBehaviorPresets())
}

func TestValidatePDBBehaviorPresets_UnknownPreset(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Name:     "pdb",
		Behavior: &types.PDBBehavior{Preset: "strict"},
	})
	err := e.validatePDBBehaviorPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown behavior.preset")
	assert.Contains(t, err.Error(), "strict")
	assert.Contains(t, err.Error(), "zero-downtime")
}

func TestValidatePDBBehaviorPresets_MixedWithMinAvailable(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Preset:       "rolling",
			MinAvailable: "1",
		},
	})
	err := e.validatePDBBehaviorPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minAvailable/maxUnavailable")
}

func TestValidatePDBBehaviorPresets_MixedWithMaxUnavailable(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Preset:         "relaxed",
			MaxUnavailable: "1",
		},
	})
	err := e.validatePDBBehaviorPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minAvailable/maxUnavailable")
}

func TestValidatePDBBehaviorPresets_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithPDB("app", types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Preset: "{{ .Spec.PDBPreset }}"},
	})
	assert.NoError(t, e.validatePDBBehaviorPresets())
}
