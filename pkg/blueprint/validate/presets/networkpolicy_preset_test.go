package presets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/inrun/pkg/types"
)

func blueprintWithNetworkPolicy(crdName string, nps ...types.NetworkPolicyTemplateSource) *checker {
	return newBlueprintExec(map[string]types.CRDEntry{
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
}

func TestValidateNetworkPolicyPresets_NoCRDs(t *testing.T) {
	e := newBlueprintExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_NoPreset(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{Name: "np"})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_ValidPreset_DenyAll(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "deny-all",
	})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_ValidPreset_AllowSameNamespace(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "allow-same-namespace",
	})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_ValidPreset_AllowDNSEgress(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "allow-dns-egress",
	})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_ValidPreset_DenyAllIngress(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Preset: "deny-all-ingress",
	})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_ValidPreset_DenyAllEgress(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Preset: "deny-all-egress",
	})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}

func TestValidateNetworkPolicyPresets_UnknownPreset(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "allow-everything",
	})
	err := e.validateNetworkPolicyPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset")
	assert.Contains(t, err.Error(), "allow-everything")
	assert.Contains(t, err.Error(), "deny-all")
}

func TestValidateNetworkPolicyPresets_MixedWithIngress(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Preset:  "deny-all",
		Ingress: []types.NetworkPolicyIngressRule{{}},
	})
	err := e.validateNetworkPolicyPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ingress/egress/policyTypes")
}

func TestValidateNetworkPolicyPresets_MixedWithEgress(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:   "np",
		Preset: "deny-all",
		Egress: []types.NetworkPolicyEgressRule{{}},
	})
	err := e.validateNetworkPolicyPresets()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ingress/egress/policyTypes")
}

func TestValidateNetworkPolicyPresets_TemplateExprSkipped(t *testing.T) {
	e := blueprintWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Preset: "{{ .Spec.NetworkPreset }}",
	})
	assert.NoError(t, e.validateNetworkPolicyPresets())
}
