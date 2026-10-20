package types

import (
	"testing"

	"github.com/intentd/kubewright/presets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresetRegistry_IsEmpty(t *testing.T) {
	assert.True(t, PresetRegistry{}.Empty())
	assert.False(t, PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{{Name: "x"}},
		},
	}.Empty())
}

func TestPresetRegistry_LookupNetworkPolicy(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{
				{Name: "allow-monitoring", PolicyTypes: []string{"Ingress"}},
			},
		},
	}
	got, found := reg.NetworkPolicy("allow-monitoring")
	require.True(t, found)
	assert.Equal(t, "allow-monitoring", got.Name)

	_, found = reg.NetworkPolicy("missing")
	assert.False(t, found)
}

func TestPresetRegistry_LookupResourceQuota(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			ResourceQuotas: []ResourceQuotaPresetDef{
				{Name: "team-medium", Hard: map[string]string{"pods": "30"}},
			},
		},
	}
	got, found := reg.ResourceQuota("team-medium")
	require.True(t, found)
	assert.Equal(t, "30", got.Hard["pods"])

	_, found = reg.ResourceQuota("missing")
	assert.False(t, found)
}

func TestPresetRegistry_LookupHPA(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			HPAs: []HPAPresetDef{
				{Name: "aggressive-scale", MaxReplicas: "20"},
			},
		},
	}
	got, found := reg.HPA("aggressive-scale")
	require.True(t, found)
	assert.Equal(t, "20", got.MaxReplicas)

	_, found = reg.HPA("missing")
	assert.False(t, found)
}

func TestPresetRegistry_LookupPDB(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			PDBs: []PDBPresetDef{
				{Name: "strict", MinAvailable: "2"},
			},
		},
	}
	got, found := reg.PDB("strict")
	require.True(t, found)
	assert.Equal(t, "2", got.MinAvailable)
}

func TestPresetRegistry_LookupRollingUpdate(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			RollingUpdates: []RollingUpdatePresetDef{
				{Name: "canary", MaxSurge: "1", MaxUnavailable: "0"},
			},
		},
	}
	got, found := reg.RollingUpdate("canary")
	require.True(t, found)
	assert.Equal(t, "1", got.MaxSurge)
}

func TestPresetRegistry_Merge_NoConflict(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{{Name: "base-policy"}},
		},
	}
	other := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{{Name: "module-policy"}},
			ResourceQuotas:  []ResourceQuotaPresetDef{{Name: "module-quota", Hard: map[string]string{"pods": "10"}}},
		},
	}
	merged, err := base.Merge(other, "module \"tenant-isolation\"")
	require.NoError(t, err)
	assert.Len(t, merged.NetworkPolicies, 2)
	assert.Len(t, merged.ResourceQuotas, 1)
}

func TestPresetRegistry_Merge_ConflictNetworkPolicy(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{{Name: "shared-policy"}},
		},
	}
	other := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{{Name: "shared-policy"}},
		},
	}
	_, err := base.Merge(other, "module \"tenant-isolation\"")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "preset conflict")
	assert.Contains(t, err.Error(), "shared-policy")
}

func TestPresetRegistry_Merge_ConflictResourceQuota(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{
			ResourceQuotas: []ResourceQuotaPresetDef{{Name: "shared", Hard: map[string]string{}}},
		},
	}
	other := PresetRegistry{
		Registry: presets.Registry{
			ResourceQuotas: []ResourceQuotaPresetDef{{Name: "shared", Hard: map[string]string{}}},
		},
	}
	_, err := base.Merge(other, "module \"quotas\"")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resourceQuotas")
}

func TestPresetRegistry_Merge_SameNameDifferentClass_Allowed(t *testing.T) {
	// "medium" in resourceQuotas and "medium" in hpa are independent — no conflict.
	base := PresetRegistry{
		Registry: presets.Registry{
			ResourceQuotas: []ResourceQuotaPresetDef{{Name: "medium", Hard: map[string]string{}}},
		},
	}
	other := PresetRegistry{
		Registry: presets.Registry{
			HPAs: []HPAPresetDef{{Name: "medium", MaxReplicas: "10"}},
		},
	}
	merged, err := base.Merge(other, "module \"sizing\"")
	require.NoError(t, err)
	assert.Len(t, merged.ResourceQuotas, 1)
	assert.Len(t, merged.HPAs, 1)
}

// ── Empty: each class ───────────────────────────────────────────────────────

func TestPresetRegistry_IsEmpty_EachClass(t *testing.T) {
	cases := []struct {
		name string
		reg  PresetRegistry
	}{
		{"ResourceQuotas", PresetRegistry{
			Registry: presets.Registry{ResourceQuotas: []ResourceQuotaPresetDef{{Name: "x"}}},
		}},
		{"LimitRanges", PresetRegistry{
			Registry: presets.Registry{LimitRanges: []LimitRangePresetDef{{Name: "x"}}},
		}},
		{"HPA", PresetRegistry{
			Registry: presets.Registry{HPAs: []HPAPresetDef{{Name: "x"}}},
		}},
		{"PDB", PresetRegistry{
			Registry: presets.Registry{PDBs: []PDBPresetDef{{Name: "x"}}},
		}},
		{"RollingUpdate", PresetRegistry{
			Registry: presets.Registry{RollingUpdates: []RollingUpdatePresetDef{{Name: "x"}}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, tc.reg.Empty())
		})
	}
}

// ── Lookup: multiple entries, returns correct one ─────────────────────────────

func TestPresetRegistry_LookupNetworkPolicy_MultipleEntries(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{
				{Name: "first", PolicyTypes: []string{"Ingress"}},
				{Name: "second", PolicyTypes: []string{"Egress"}},
				{Name: "third", PolicyTypes: []string{"Ingress", "Egress"}},
			},
		},
	}
	got, found := reg.NetworkPolicy("second")
	require.True(t, found)
	assert.Equal(t, []string{"Egress"}, got.PolicyTypes)
}

func TestPresetRegistry_LookupLimitRange(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			LimitRanges: []LimitRangePresetDef{
				{
					Name: "default-limits",
					Limits: []LimitRangeItem{
						{Type: "Container", Default: map[string]string{"cpu": "500m", "memory": "256Mi"}},
					},
				},
			},
		},
	}
	got, found := reg.LimitRange("default-limits")
	require.True(t, found)
	assert.Len(t, got.Limits, 1)
	assert.Equal(t, "500m", got.Limits[0].Default["cpu"])

	_, found = reg.LimitRange("missing")
	assert.False(t, found)
}

func TestPresetRegistry_LookupHPA_WithMinReplicas(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			HPAs: []HPAPresetDef{
				{Name: "scaled", MinReplicas: "2", MaxReplicas: "{{ .spec.maxReplicas }}", TargetCPUUtilizationPercentage: "70"},
			},
		},
	}
	got, found := reg.HPA("scaled")
	require.True(t, found)
	assert.Equal(t, "2", got.MinReplicas)
	assert.Equal(t, "{{ .spec.maxReplicas }}", got.MaxReplicas)
	assert.Equal(t, "70", got.TargetCPUUtilizationPercentage)
}

func TestPresetRegistry_LookupPDB_MaxUnavailable(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			PDBs: []PDBPresetDef{
				{Name: "relaxed", MaxUnavailable: "{{ .spec.maxUnavailable }}"},
			},
		},
	}
	got, found := reg.PDB("relaxed")
	require.True(t, found)
	assert.Equal(t, "{{ .spec.maxUnavailable }}", got.MaxUnavailable)
}

func TestPresetRegistry_LookupRollingUpdate_BothFields(t *testing.T) {
	reg := PresetRegistry{
		Registry: presets.Registry{
			RollingUpdates: []RollingUpdatePresetDef{
				{Name: "gradual", MaxSurge: "{{ .spec.surge }}", MaxUnavailable: "0"},
			},
		},
	}
	got, found := reg.RollingUpdate("gradual")
	require.True(t, found)
	assert.Equal(t, "{{ .spec.surge }}", got.MaxSurge)
	assert.Equal(t, "0", got.MaxUnavailable)
}

// ── Merge: conflict for every class ──────────────────────────────────────────

func TestPresetRegistry_Merge_ConflictHPA(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{HPAs: []HPAPresetDef{{Name: "clash"}}},
	}
	other := PresetRegistry{
		Registry: presets.Registry{HPAs: []HPAPresetDef{{Name: "clash"}}},
	}
	_, err := base.Merge(other, "module \"scaling\"")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hpa")
	assert.Contains(t, err.Error(), "clash")
}

func TestPresetRegistry_Merge_ConflictPDB(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{PDBs: []PDBPresetDef{{Name: "clash"}}},
	}
	other := PresetRegistry{
		Registry: presets.Registry{PDBs: []PDBPresetDef{{Name: "clash"}}},
	}
	_, err := base.Merge(other, "module \"disruption\"")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pdb")
}

func TestPresetRegistry_Merge_ConflictRollingUpdate(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{RollingUpdates: []RollingUpdatePresetDef{{Name: "clash"}}},
	}
	other := PresetRegistry{
		Registry: presets.Registry{RollingUpdates: []RollingUpdatePresetDef{{Name: "clash"}}},
	}
	_, err := base.Merge(other, "module \"rollout\"")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollingUpdate")
}

func TestPresetRegistry_Merge_ConflictLimitRange(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{LimitRanges: []LimitRangePresetDef{{Name: "clash"}}},
	}
	other := PresetRegistry{
		Registry: presets.Registry{LimitRanges: []LimitRangePresetDef{{Name: "clash"}}},
	}
	_, err := base.Merge(other, "module \"limits\"")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limitRanges")
}

func TestPresetRegistry_Merge_EmptyBase(t *testing.T) {
	other := PresetRegistry{
		Registry: presets.Registry{
			NetworkPolicies: []NetworkPolicyPresetDef{{Name: "np"}},
			HPAs:            []HPAPresetDef{{Name: "hpa", MaxReplicas: "5"}},
		},
	}
	merged, err := PresetRegistry{}.Merge(other, "module \"full\"")
	require.NoError(t, err)
	assert.Len(t, merged.NetworkPolicies, 1)
	assert.Len(t, merged.HPAs, 1)
}

func TestPresetRegistry_Merge_EmptyOther(t *testing.T) {
	base := PresetRegistry{
		Registry: presets.Registry{
			PDBs: []PDBPresetDef{{Name: "strict", MinAvailable: "2"}},
		},
	}
	merged, err := base.Merge(PresetRegistry{}, "module \"empty\"")
	require.NoError(t, err)
	assert.Len(t, merged.PDBs, 1)
}

// ── NetworkPolicyPresetDef fields ───────────────────────────────────────────

func TestNetworkPolicyPresetDef_FullFields(t *testing.T) {
	def := NetworkPolicyPresetDef{
		Name:        "allow-monitoring",
		Description: "Allows ingress from the platform monitoring namespace",
		PodSelector: map[string]interface{}{"app": "my-app"},
		Ingress: []NetworkPolicyIngressRule{
			{From: []NetworkPolicyPeer{{NamespaceSelector: map[string]string{"team": "platform"}}}},
		},
		PolicyTypes: []string{"Ingress"},
	}
	assert.Equal(t, "allow-monitoring", def.Name)
	assert.Len(t, def.Ingress, 1)
	assert.Equal(t, []string{"Ingress"}, def.PolicyTypes)
}

// ── ResourceQuotaPresetDef: template expressions ────────────────────────────

func TestResourceQuotaPresetDef_TemplateExpressions(t *testing.T) {
	def := ResourceQuotaPresetDef{
		Name: "dynamic",
		Hard: map[string]string{
			"pods":   "{{ .spec.maxPods }}",
			"cpu":    "{{ .spec.cpuLimit }}",
			"memory": "{{ .spec.memLimit }}",
		},
	}
	reg := PresetRegistry{
		Registry: presets.Registry{ResourceQuotas: []ResourceQuotaPresetDef{def}},
	}
	got, found := reg.ResourceQuota("dynamic")
	require.True(t, found)
	assert.Equal(t, "{{ .spec.maxPods }}", got.Hard["pods"])
}
