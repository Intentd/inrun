package profiles_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

func TestNetworkPolicyProfiles(t *testing.T) {
	var empty *types.ProfileRegistry

	tests := []struct {
		name          string
		profile       string
		expectErr     bool
		expectIngress int
		expectEgress  int
		expectTypes   []string
	}{
		{"deny-all", "deny-all", false, 0, 0, []string{"Ingress", "Egress"}},
		{"deny-all-ingress", "deny-all-ingress", false, 0, -1, []string{"Ingress"}},
		{"deny-all-egress", "deny-all-egress", false, -1, 0, []string{"Egress"}},
		{"allow-same-namespace", "allow-same-namespace", false, 1, -1, []string{"Ingress"}},
		{"allow-dns-egress", "allow-dns-egress", false, -1, 1, []string{"Egress"}},
		{"unknown profile fails fast", "open-all", true, 0, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := profiles.ApplyNetworkPolicyProfile(tt.profile, empty)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectIngress >= 0 && len(exp.Ingress) != tt.expectIngress {
				t.Errorf("ingress rules: want %d got %d", tt.expectIngress, len(exp.Ingress))
			}
			if tt.expectEgress >= 0 && len(exp.Egress) != tt.expectEgress {
				t.Errorf("egress rules: want %d got %d", tt.expectEgress, len(exp.Egress))
			}
			if len(tt.expectTypes) > 0 {
				for i, pt := range tt.expectTypes {
					if i >= len(exp.PolicyTypes) || exp.PolicyTypes[i] != pt {
						t.Errorf("policyTypes[%d]: want %q got %v", i, pt, exp.PolicyTypes)
					}
				}
			}
		})
	}
}

func TestNetworkPolicyCaseInsensitive(t *testing.T) {
	var empty *types.ProfileRegistry
	for _, name := range []string{"DENY-ALL", "Deny-All-Ingress", "ALLOW-DNS-EGRESS"} {
		_, err := profiles.ApplyNetworkPolicyProfile(name, empty)
		if err != nil {
			t.Errorf("expected %q to be case-insensitive, got error: %v", name, err)
		}
	}
}

func TestNetworkPolicyProfileUserDefined(t *testing.T) {
	reg := types.ProfileRegistry{
		NetworkPolicies: []types.NetworkPolicyProfileDef{
			{
				Name:        "allow-internal",
				PolicyTypes: []string{"Ingress"},
				Ingress: []types.NetworkPolicyIngressRule{
					{From: []types.NetworkPolicyPeer{{PodSelector: map[string]string{"app": "frontend"}}}},
				},
			},
		},
	}

	exp, err := profiles.ApplyNetworkPolicyProfile("allow-internal", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exp.Ingress) != 1 {
		t.Fatalf("want 1 ingress rule got %d", len(exp.Ingress))
	}
	if len(exp.PolicyTypes) != 1 || exp.PolicyTypes[0] != "Ingress" {
		t.Errorf("policyTypes: want [Ingress] got %v", exp.PolicyTypes)
	}
}

func TestIsValidNetworkPolicyProfile(t *testing.T) {
	valid := []string{"deny-all", "deny-all-ingress", "deny-all-egress", "allow-same-namespace", "allow-dns-egress"}
	for _, name := range valid {
		if !profiles.IsValidNetworkPolicyProfile(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "open-all", "allow-all"}
	for _, name := range invalid {
		if profiles.IsValidNetworkPolicyProfile(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
