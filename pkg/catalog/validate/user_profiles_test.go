package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

func catalogWithProfiles(reg types.ProfileRegistry) *executor {
	return newExec(&catalog.Catalog{Profiles: reg})
}

func catalogWithProfilesAndNP(reg types.ProfileRegistry, crdName string, nps ...types.NetworkPolicyTemplateSource) *executor {
	k := catalog.NewCatalogForTest(map[string]types.CRDEntry{
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
	k.Profiles = reg
	return newExec(k)
}

func catalogWithProfilesAndRQ(reg types.ProfileRegistry, crdName string, rqs ...types.ResourceQuotaTemplateSource) *executor {
	k := catalog.NewCatalogForTest(map[string]types.CRDEntry{
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
	k.Profiles = reg
	return newExec(k)
}

// ── validateUserProfiles ──────────────────────────────────────────────────────

func TestValidateUserProfiles_Empty(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_ValidNetworkPolicy(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_ValidResourceQuota(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "team-medium", Hard: map[string]string{"pods": "30", "cpu": "6"}},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_ValidHPA(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		HPA: []types.HPAProfileDef{
			{Name: "aggressive-scale", MinReplicas: "2", MaxReplicas: "20"},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_ValidPDB(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		PDB: []types.PDBProfileDef{
			{Name: "strict", MinAvailable: "2"},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_ValidRollingUpdate(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		RollingUpdate: []types.RollingUpdateProfileDef{
			{Name: "canary", MaxSurge: "1", MaxUnavailable: "0"},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_TemplateExpressionInHard(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "dynamic", Hard: map[string]string{"pods": "{{ .spec.maxPods }}"}},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

func TestValidateUserProfiles_DuplicateNetworkPolicyName(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "allow-monitoring"},
			{Name: "allow-monitoring"},
		},
	})
	err := k.validateUserProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate profile name")
	assert.Contains(t, err.Error(), "allow-monitoring")
}

func TestValidateUserProfiles_DuplicateResourceQuotaName(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "team-medium", Hard: map[string]string{"pods": "10"}},
			{Name: "team-medium", Hard: map[string]string{"pods": "20"}},
		},
	})
	err := k.validateUserProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate profile name")
}

func TestValidateUserProfiles_MissingName(t *testing.T) {
	k := catalogWithProfiles(types.ProfileRegistry{
		PDB: []types.PDBProfileDef{
			{MinAvailable: "1"},
		},
	})
	err := k.validateUserProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing a name")
}

func TestValidateUserProfiles_ShadowingBuiltinIsAllowed(t *testing.T) {
	// Shadowing a built-in produces a warning but is not an error.
	k := catalogWithProfiles(types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "deny-all", PolicyTypes: []string{"Ingress", "Egress"}},
		},
	})
	assert.NoError(t, k.validateUserProfiles())
}

// ── user profile used in networkPolicy reference ──────────────────────────────

func TestValidateNetworkPolicyProfiles_UserDefinedProfileAccepted(t *testing.T) {
	reg := types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
		},
	}
	k := catalogWithProfilesAndNP(reg, "app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "allow-monitoring",
	})
	require.NoError(t, k.validateUserProfiles())
	assert.NoError(t, k.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_UnknownProfileStillRejected(t *testing.T) {
	k := catalogWithProfilesAndNP(types.ProfileRegistry{}, "app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "custom-unknown",
	})
	assert.Error(t, k.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_UserProfileShadowsBuiltin(t *testing.T) {
	reg := types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{Name: "deny-all", PolicyTypes: []string{"Ingress"}},
		},
	}
	k := catalogWithProfilesAndNP(reg, "app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "deny-all",
	})
	require.NoError(t, k.validateUserProfiles())
	assert.NoError(t, k.validateNetworkPolicyProfiles())
}

// ── user profile used in resourceQuota reference ──────────────────────────────

func TestValidateResourceQuotaProfiles_UserDefinedProfileAccepted(t *testing.T) {
	reg := types.ProfileRegistry{
		ResourceQuotas: []types.ResourceQuotaProfileDef{
			{Name: "team-medium", Hard: map[string]string{"pods": "30"}},
		},
	}
	k := catalogWithProfilesAndRQ(reg, "app", types.ResourceQuotaTemplateSource{
		Name:    "rq",
		Profile: "team-medium",
	})
	require.NoError(t, k.validateUserProfiles())
	assert.NoError(t, k.validateResourceQuotaProfiles())
}

func TestValidateResourceQuotaProfiles_UnknownProfileStillRejected(t *testing.T) {
	k := catalogWithProfilesAndRQ(types.ProfileRegistry{}, "app", types.ResourceQuotaTemplateSource{
		Name:    "rq",
		Profile: "custom-unknown",
	})
	assert.Error(t, k.validateResourceQuotaProfiles())
}
