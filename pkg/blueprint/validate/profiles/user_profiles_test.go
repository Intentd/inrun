package profiles

import (
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func blueprintWithProfiles(reg types.ProfileRegistry) *checker {
	return newExec(&blueprint.Blueprint{Profiles: reg})
}

func blueprintWithProfilesAndNP(reg types.ProfileRegistry, crdName string, nps ...types.NetworkPolicyTemplateSource) *checker {
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
	bp.Profiles = reg
	return newExec(bp)
}

func blueprintWithProfilesAndRQ(reg types.ProfileRegistry, crdName string, rqs ...types.ResourceQuotaTemplateSource) *checker {
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
	bp.Profiles = reg
	return newExec(bp)
}

// ── validateUserProfiles ──────────────────────────────────────────────────────

func TestValidateUserProfiles_Empty(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_ValidNetworkPolicy(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_ValidResourceQuota(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "team-medium", Hard: map[string]string{"pods": "30", "cpu": "6"}},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_ValidHPA(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		HPA: []types.HPAProfileDef{
			{Name: "aggressive-scale", MinReplicas: "2", MaxReplicas: "20"},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_ValidPDB(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		PDB: []types.PDBProfileDef{
			{Name: "strict", MinAvailable: "2"},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_ValidRollingUpdate(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		RollingUpdate: []types.RollingUpdateProfileDef{
			{Name: "canary", MaxSurge: "1", MaxUnavailable: "0"},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_TemplateExpressionInHard(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "dynamic", Hard: map[string]string{"pods": "{{ .spec.maxPods }}"}},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

func TestValidateUserProfiles_DuplicateNetworkPolicyName(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "allow-monitoring"},
			{Name: "allow-monitoring"},
		},
	})
	err := e.validateUserProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate profile name")
	assert.Contains(t, err.Error(), "allow-monitoring")
}

func TestValidateUserProfiles_DuplicateResourceQuotaName(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "team-medium", Hard: map[string]string{"pods": "10"}},
			{Name: "team-medium", Hard: map[string]string{"pods": "20"}},
		},
	})
	err := e.validateUserProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate profile name")
}

func TestValidateUserProfiles_MissingName(t *testing.T) {
	e := blueprintWithProfiles(types.ProfileRegistry{
		PDB: []types.PDBProfileDef{
			{MinAvailable: "1"},
		},
	})
	err := e.validateUserProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing a name")
}

func TestValidateUserProfiles_ShadowingBuiltinIsAllowed(t *testing.T) {
	// Shadowing a built-in produces a warning but is not an error.
	e := blueprintWithProfiles(types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "deny-all", PolicyTypes: []string{"Ingress", "Egress"}},
		},
	})
	assert.NoError(t, e.validateUserProfiles())
}

// ── user profile used in networkPolicy reference ──────────────────────────────

func TestValidateNetworkPolicyProfiles_UserDefinedProfileAccepted(t *testing.T) {
	reg := types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
		},
	}
	e := blueprintWithProfilesAndNP(reg, "app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "allow-monitoring",
	})
	require.NoError(t, e.validateUserProfiles())
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_UnknownProfileStillRejected(t *testing.T) {
	e := blueprintWithProfilesAndNP(types.ProfileRegistry{}, "app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "custom-unknown",
	})
	assert.Error(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_UserProfileShadowsBuiltin(t *testing.T) {
	reg := types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "deny-all", PolicyTypes: []string{"Ingress"}},
		},
	}
	e := blueprintWithProfilesAndNP(reg, "app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "deny-all",
	})
	require.NoError(t, e.validateUserProfiles())
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

// ── user profile used in resourceQuota reference ──────────────────────────────

func TestValidateResourceQuotaProfiles_UserDefinedProfileAccepted(t *testing.T) {
	reg := types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "team-medium", Hard: map[string]string{"pods": "30"}},
		},
	}
	e := blueprintWithProfilesAndRQ(reg, "app", types.ResourceQuotaTemplateSource{
		Name:    "rq",
		Profile: "team-medium",
	})
	require.NoError(t, e.validateUserProfiles())
	assert.NoError(t, e.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_UnknownProfileStillRejected(t *testing.T) {
	e := blueprintWithProfilesAndRQ(types.ProfileRegistry{}, "app", types.ResourceQuotaTemplateSource{
		Name:    "rq",
		Profile: "custom-unknown",
	})
	assert.Error(t, e.validateResourceQuotaProfiles())
}
