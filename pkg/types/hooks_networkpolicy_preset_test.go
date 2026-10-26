package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func crdWithNetworkPolicyOnCreate(nps ...types.NetworkPolicyTemplateSource) types.CRDEntry {
	return types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnCreate: &types.HookTemplates{
					NetworkPolicies: nps,
				},
			},
		},
	}
}

func TestCollectNetworkPolicyPresetEntries_Empty(t *testing.T) {
	c := types.CRDEntry{}
	assert.Empty(t, c.CollectNetworkPolicyPresetEntries())
}

func TestCollectNetworkPolicyPresetEntries_NoPreset(t *testing.T) {
	c := crdWithNetworkPolicyOnCreate(types.NetworkPolicyTemplateSource{Name: "allow"})
	assert.Empty(t, c.CollectNetworkPolicyPresetEntries())
}

func TestCollectNetworkPolicyPresetEntries_PresetReturned(t *testing.T) {
	c := crdWithNetworkPolicyOnCreate(types.NetworkPolicyTemplateSource{
		Name:   "default",
		Preset: "deny-all",
	})
	entries := c.CollectNetworkPolicyPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onCreate", entries[0].Phase)
	assert.Equal(t, "default", entries[0].ResourceName)
	assert.Equal(t, "deny-all", entries[0].Preset)
	assert.False(t, entries[0].Mixed)
}

func TestCollectNetworkPolicyPresetEntries_Mixed_Ingress(t *testing.T) {
	c := crdWithNetworkPolicyOnCreate(types.NetworkPolicyTemplateSource{
		Name:    "np",
		Preset:  "deny-all",
		Ingress: []types.NetworkPolicyIngressRule{{}},
	})
	entries := c.CollectNetworkPolicyPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectNetworkPolicyPresetEntries_Mixed_Egress(t *testing.T) {
	c := crdWithNetworkPolicyOnCreate(types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "allow-dns-egress",
		Egress: []types.NetworkPolicyEgressRule{{}},
	})
	entries := c.CollectNetworkPolicyPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectNetworkPolicyPresetEntries_Mixed_PolicyTypes(t *testing.T) {
	c := crdWithNetworkPolicyOnCreate(types.NetworkPolicyTemplateSource{
		Name:        "np",
		Preset:      "deny-all",
		PolicyTypes: []string{"Ingress"},
	})
	entries := c.CollectNetworkPolicyPresetEntries()
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Mixed)
}

func TestCollectNetworkPolicyPresetEntries_TemplateExpr(t *testing.T) {
	c := crdWithNetworkPolicyOnCreate(types.NetworkPolicyTemplateSource{
		Preset: "{{ .Spec.Preset }}",
	})
	entries := c.CollectNetworkPolicyPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "{{ .Spec.Preset }}", entries[0].Preset)
}

func TestCollectNetworkPolicyPresetEntries_OnReconcile(t *testing.T) {
	c := types.CRDEntry{
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				OnReconcile: &types.HookTemplates{
					NetworkPolicies: []types.NetworkPolicyTemplateSource{
						{Name: "np", Preset: "allow-same-namespace"},
					},
				},
			},
		},
	}
	entries := c.CollectNetworkPolicyPresetEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "onReconcile", entries[0].Phase)
}
