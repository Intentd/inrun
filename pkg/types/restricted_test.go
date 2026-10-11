// pkg/types/types_test/restricted_test.go
package types_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func TestRestrictedNamespaces_IsRestricted(t *testing.T) {
	r := types.RestrictedNamespaces{
		"kube-system",
		"cert-manager",
		"kube-*",
		"*-system",
	}

	tests := []struct {
		namespace string
		want      bool
	}{
		// exact matches
		{"kube-system", true},
		{"cert-manager", true},
		// prefix wildcard: kube-*
		{"kube-public", true},
		{"kube-node-lease", true},
		// suffix wildcard: *-system
		{"logging-system", true},
		{"monitoring-system", true},
		// no match
		{"default", false},
		{"production", false},
		{"my-app", false},
		// edge: kube-system matches both exact and prefix — still true
		{"kube-anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := r.IsRestricted(tt.namespace)
			if got != tt.want {
				t.Errorf("IsRestricted(%q): expected %v, got %v", tt.namespace, tt.want, got)
			}
		})
	}
}

func TestRestrictedNamespaces_Empty(t *testing.T) {
	r := types.RestrictedNamespaces{}
	if r.IsRestricted("kube-system") {
		t.Error("empty restrictions should never match")
	}
	if r.IsRestricted("anything") {
		t.Error("empty restrictions should never match")
	}
}

func TestRestrictedNamespaces_Merge_Deduplication(t *testing.T) {
	a := types.RestrictedNamespaces{"kube-system", "cert-manager"}
	b := types.RestrictedNamespaces{"cert-manager", "monitoring"} // cert-manager duplicated

	merged := a.Merge(b)

	if len(merged) != 3 {
		t.Errorf("expected 3 entries after dedup, got %d: %v", len(merged), merged)
	}

	if !merged.IsRestricted("kube-system") {
		t.Error("kube-system should be restricted after merge")
	}
	if !merged.IsRestricted("cert-manager") {
		t.Error("cert-manager should be restricted after merge")
	}
	if !merged.IsRestricted("monitoring") {
		t.Error("monitoring should be restricted after merge")
	}
}

func TestRestrictedNamespaces_Merge_Additive(t *testing.T) {
	// Stack-level restrictions
	platform := types.RestrictedNamespaces{"kube-system", "cert-manager"}
	// CRD-level adds more — cannot remove platform restrictions
	crd := types.RestrictedNamespaces{"monitoring"}

	merged := platform.Merge(crd)

	if !merged.IsRestricted("kube-system") {
		t.Error("platform restriction kube-system must survive merge")
	}
	if !merged.IsRestricted("monitoring") {
		t.Error("CRD restriction monitoring must be present after merge")
	}
}

func TestRestrictedNamespaces_Merge_WithWildcards(t *testing.T) {
	a := types.RestrictedNamespaces{"kube-*"}
	b := types.RestrictedNamespaces{"*-system"}

	merged := a.Merge(b)

	if !merged.IsRestricted("kube-public") {
		t.Error("kube-public should match kube-* after merge")
	}
	if !merged.IsRestricted("logging-system") {
		t.Error("logging-system should match *-system after merge")
	}
	if merged.IsRestricted("default") {
		t.Error("default should not match any pattern")
	}
}
