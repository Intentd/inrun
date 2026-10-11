package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithPDB(crdName string, pdbs ...types.PDBTemplateSource) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
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

func TestValidatePDBBehaviorProfiles_NoCRDs(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validatePDBBehaviorProfiles())
}

func TestValidatePDBBehaviorProfiles_NoProfile(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{Name: "pdb"})
	assert.NoError(t, e.validatePDBBehaviorProfiles())
}

func TestValidatePDBBehaviorProfiles_ValidProfile_ZeroDowntime(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Name:     "pdb",
		Behavior: &types.PDBBehavior{Profile: "zero-downtime"},
	})
	assert.NoError(t, e.validatePDBBehaviorProfiles())
}

func TestValidatePDBBehaviorProfiles_ValidProfile_Rolling(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Profile: "rolling"},
	})
	assert.NoError(t, e.validatePDBBehaviorProfiles())
}

func TestValidatePDBBehaviorProfiles_ValidProfile_Relaxed(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Profile: "relaxed"},
	})
	assert.NoError(t, e.validatePDBBehaviorProfiles())
}

func TestValidatePDBBehaviorProfiles_UnknownProfile(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Name:     "pdb",
		Behavior: &types.PDBBehavior{Profile: "strict"},
	})
	err := e.validatePDBBehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown behavior.profile")
	assert.Contains(t, err.Error(), "strict")
	assert.Contains(t, err.Error(), "zero-downtime")
}

func TestValidatePDBBehaviorProfiles_MixedWithMinAvailable(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Profile:      "rolling",
			MinAvailable: "1",
		},
	})
	err := e.validatePDBBehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minAvailable/maxUnavailable")
}

func TestValidatePDBBehaviorProfiles_MixedWithMaxUnavailable(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Name: "pdb",
		Behavior: &types.PDBBehavior{
			Profile:        "relaxed",
			MaxUnavailable: "1",
		},
	})
	err := e.validatePDBBehaviorProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minAvailable/maxUnavailable")
}

func TestValidatePDBBehaviorProfiles_TemplateExprSkipped(t *testing.T) {
	e := catalogWithPDB("app", types.PDBTemplateSource{
		Behavior: &types.PDBBehavior{Profile: "{{ .Spec.PDBProfile }}"},
	})
	assert.NoError(t, e.validatePDBBehaviorProfiles())
}
