package resources

import (
	"testing"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

func TestResolveNetworkPolicy_PodSelector(t *testing.T) {
	r := template.NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "platform-egress"},
	})
	got, err := ResolveNetworkPolicy(r, types.NetworkPolicyTemplateSource{
		PodSelector: map[string]string{"policy": "{{ .metadata.name }}", "tier": "web"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.PodSelector["policy"] != "platform-egress" || got.PodSelector["tier"] != "web" {
		t.Fatalf("podSelector not resolved: %v", got.PodSelector)
	}
}

func TestResolveNetworkPolicy_Rules(t *testing.T) {
	r := template.NewResolverFromMap(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "team"},
		"spec":     map[string]interface{}{"port": "8080", "cidr": "10.0.0.0/8"},
	})
	got, err := ResolveNetworkPolicy(r, types.NetworkPolicyTemplateSource{
		Ingress: []types.NetworkPolicyIngressRule{{
			From: []types.NetworkPolicyPeer{
				{PodSelector: map[string]string{"app": "{{ .metadata.name }}"}},
				{IPBlock: &types.NetworkPolicyIPBlock{CIDR: "{{ .spec.cidr }}", Except: []string{"10.1.0.0/16"}}},
			},
			Ports: []types.NetworkPolicyPort{{Protocol: "TCP", Port: "{{ .spec.port }}"}},
		}},
		Egress: []types.NetworkPolicyEgressRule{{
			To: []types.NetworkPolicyPeer{{NamespaceSelector: map[string]string{"team": "{{ .metadata.namespace }}"}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	in := got.Ingress[0]
	if in.From[0].PodSelector["app"] != "web" || in.From[1].IPBlock.CIDR != "10.0.0.0/8" || in.Ports[0].Port != "8080" {
		t.Fatalf("ingress not resolved: %+v", in)
	}
	if in.From[0].NamespaceSelector != nil {
		t.Fatal("an absent namespaceSelector must stay absent: empty selects every namespace")
	}
	if got.Egress[0].To[0].NamespaceSelector["team"] != "team" {
		t.Fatalf("egress not resolved: %+v", got.Egress[0])
	}
}
