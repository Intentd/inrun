package validate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogWithNetworkPolicy(crdName string, nps ...types.NetworkPolicyTemplateSource) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
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

func TestValidateNetworkPolicyProfiles_NoCRDs(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_NoProfile(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{Name: "np"})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_ValidProfile_DenyAll(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "deny-all",
	})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_ValidProfile_AllowSameNamespace(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "allow-same-namespace",
	})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_ValidProfile_AllowDNSEgress(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "allow-dns-egress",
	})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_ValidProfile_DenyAllIngress(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Profile: "deny-all-ingress",
	})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_ValidProfile_DenyAllEgress(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Profile: "deny-all-egress",
	})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}

func TestValidateNetworkPolicyProfiles_UnknownProfile(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "allow-everything",
	})
	err := e.validateNetworkPolicyProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown profile")
	assert.Contains(t, err.Error(), "allow-everything")
	assert.Contains(t, err.Error(), "deny-all")
}

func TestValidateNetworkPolicyProfiles_MixedWithIngress(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "deny-all",
		Ingress: []types.NetworkPolicyIngressRule{{}},
	})
	err := e.validateNetworkPolicyProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ingress/egress/policyTypes")
}

func TestValidateNetworkPolicyProfiles_MixedWithEgress(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Name:    "np",
		Profile: "deny-all",
		Egress:  []types.NetworkPolicyEgressRule{{}},
	})
	err := e.validateNetworkPolicyProfiles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ingress/egress/policyTypes")
}

func TestValidateNetworkPolicyProfiles_TemplateExprSkipped(t *testing.T) {
	e := catalogWithNetworkPolicy("app", types.NetworkPolicyTemplateSource{
		Profile: "{{ .Spec.NetworkProfile }}",
	})
	assert.NoError(t, e.validateNetworkPolicyProfiles())
}
