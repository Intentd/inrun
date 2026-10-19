package presets

import (
	"testing"

	respresets "github.com/intentd/inrun/pkg/resources/presets"

	"github.com/intentd/inrun/pkg/blueprint"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

// helpers

func blueprintWithPresets(reg types.PresetRegistry) *checker {
	return newExec(&blueprint.Blueprint{Presets: reg})
}

func blueprintWithPresetsAndNP(reg types.PresetRegistry, crdName string, nps ...types.NetworkPolicyTemplateSource) *checker {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		crdName: {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						NetworkPolicies: nps,
					},
				},
			},
		},
	})
	bp.Presets = reg
	return newExec(bp)
}

func blueprintWithPresetsAndRQ(reg types.PresetRegistry, crdName string, rqs ...types.ResourceQuotaTemplateSource) *checker {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		crdName: {
			OperatorBox: &types.OperatorBoxConfig{
				Reconcile: &types.ReconcileConfig{
					OnCreate: &types.HookTemplates{
						ResourceQuotas: rqs,
					},
				},
			},
		},
	})
	bp.Presets = reg
	return newExec(bp)
}

// ── validateUserPresets ──────────────────────────────────────────────────────

func TestValidateUserPresets_Empty(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_ValidNetworkPolicy(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			NetworkPolicies: []types.NetworkPolicyPresetDef{
				{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_ValidResourceQuota(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			ResourceQuotas: []types.ResourceQuotaPresetDef{
				{Name: "team-medium", Hard: map[string]string{"pods": "30", "cpu": "6"}},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_ValidHPA(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			HPAs: []types.HPAPresetDef{
				{Name: "aggressive-scale", MinReplicas: "2", MaxReplicas: "20"},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_ValidPDB(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			PDBs: []types.PDBPresetDef{
				{Name: "strict", MinAvailable: "2"},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_ValidRollingUpdate(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			RollingUpdates: []types.RollingUpdatePresetDef{
				{Name: "canary", MaxSurge: "1", MaxUnavailable: "0"},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_TemplateExpressionInHard(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			ResourceQuotas: []types.ResourceQuotaPresetDef{
				{Name: "dynamic", Hard: map[string]string{"pods": "{{ .spec.maxPods }}"}},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

func TestValidateUserPresets_DuplicateNetworkPolicyName(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			NetworkPolicies: []types.NetworkPolicyPresetDef{
				{Name: "allow-monitoring"},
				{Name: "allow-monitoring"},
			},
		},
	})
	err := e.validateUserPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate preset name")
	assert.Contains(t, err.Error(), "allow-monitoring")
}

func TestValidateUserPresets_DuplicateResourceQuotaName(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			ResourceQuotas: []types.ResourceQuotaPresetDef{
				{Name: "team-medium", Hard: map[string]string{"pods": "10"}},
				{Name: "team-medium", Hard: map[string]string{"pods": "20"}},
			},
		},
	})
	err := e.validateUserPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate preset name")
}

func TestValidateUserPresets_MissingName(t *testing.T) {
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			PDBs: []types.PDBPresetDef{
				{MinAvailable: "1"},
			},
		},
	})
	err := e.validateUserPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing a name")
}

func TestValidateUserPresets_ShadowingBuiltinIsAllowed(t *testing.T) {
	// Shadowing a built-in produces a warning but is not an error.
	e := blueprintWithPresets(types.PresetRegistry{
		Registry: respresets.Registry{
			NetworkPolicies: []types.NetworkPolicyPresetDef{
				{Name: "deny-all", PolicyTypes: []string{"Ingress", "Egress"}},
			},
		},
	})
	assert.NoError(t, e.validateUserPresets())
}

// ── user preset used in networkPolicy reference ──────────────────────────────

func TestValidateNetworkPolicyPresets_UserDefinedPresetAccepted(t *testing.T) {
	reg := types.PresetRegistry{
		Registry: respresets.Registry{
			NetworkPolicies: []types.NetworkPolicyPresetDef{
				{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
			},
		},
	}
	e := blueprintWithPresetsAndNP(reg, "app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "allow-monitoring",
	})
	require.NoError(t, e.validateUserPresets())
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_UnknownPresetStillRejected(t *testing.T) {
	e := blueprintWithPresetsAndNP(types.PresetRegistry{}, "app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "custom-unknown",
	})
	assert.Error(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_UserPresetShadowsBuiltin(t *testing.T) {
	reg := types.PresetRegistry{
		Registry: respresets.Registry{
			NetworkPolicies: []types.NetworkPolicyPresetDef{
				{Name: "deny-all", PolicyTypes: []string{"Ingress"}},
			},
		},
	}
	e := blueprintWithPresetsAndNP(reg, "app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "deny-all",
	})
	require.NoError(t, e.validateUserPresets())
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

// ── user preset used in resourceQuota reference ──────────────────────────────

func TestValidateResourceQuotaPresets_UserDefinedPresetAccepted(t *testing.T) {
	reg := types.PresetRegistry{
		Registry: respresets.Registry{
			ResourceQuotas: []types.ResourceQuotaPresetDef{
				{Name: "team-medium", Hard: map[string]string{"pods": "30"}},
			},
		},
	}
	e := blueprintWithPresetsAndRQ(reg, "app", types.ResourceQuotaTemplateSource{
		Name:   "rq",
		Preset: "team-medium",
	})
	require.NoError(t, e.validateUserPresets())
	assert.NoError(t, e.validateResourceQuotaPresets())
}

func TestValidateResourceQuotaPresets_UnknownPresetStillRejected(t *testing.T) {
	e := blueprintWithPresetsAndRQ(types.PresetRegistry{}, "app", types.ResourceQuotaTemplateSource{
		Name:   "rq",
		Preset: "custom-unknown",
	})
	assert.Error(t, e.validateResourceQuotaPresets())
}
