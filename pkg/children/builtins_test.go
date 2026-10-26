// pkg/children/builtins_test.go
package children

import (
	"strings"
	"testing"
)

// ── LookupBuiltIn ─────────────────────────────────────────────────────────────

func TestLookupBuiltIn_CoreGroup(t *testing.T) {
	tests := []struct {
		kind           string
		expectedPlural string
		expectedGroup  string
		expectedPKG    string
		namespaced     bool
	}{
		{"Pod", "pods", "", "core", true},
		{"Service", "services", "", "core", true},
		{"ConfigMap", "configmaps", "", "core", true},
		{"Secret", "secrets", "", "core", true},
		{"Namespace", "namespaces", "", "core", false},
		{"ServiceAccount", "serviceaccounts", "", "core", true},
		{"PersistentVolumeClaim", "persistentvolumeclaims", "", "core", true},
		{"PersistentVolume", "persistentvolumes", "", "core", false},
		{"Node", "nodes", "", "core", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := LookupBuiltIn(tt.kind)
			if !result.Found() {
				t.Fatalf("expected %q to be found in built-in registry", tt.kind)
			}
			if result.builtIn.Plural != tt.expectedPlural {
				t.Errorf("plural: expected %q, got %q", tt.expectedPlural, result.builtIn.Plural)
			}
			if result.builtIn.Group != tt.expectedGroup {
				t.Errorf("group: expected %q, got %q", tt.expectedGroup, result.builtIn.Group)
			}
			if result.DisplayGroup() != tt.expectedPKG {
				t.Errorf("display group: expected %q, got %q", tt.expectedPKG, result.DisplayGroup())
			}
			if result.builtIn.Namespaced != tt.namespaced {
				t.Errorf("namespaced: expected %v, got %v", tt.namespaced, result.builtIn.Namespaced)
			}
		})
	}
}

func TestLookupBuiltIn_AppsGroup(t *testing.T) {
	tests := []struct {
		kind   string
		plural string
	}{
		{"Deployment", "deployments"},
		{"StatefulSet", "statefulsets"},
		{"DaemonSet", "daemonsets"},
		{"ReplicaSet", "replicasets"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := LookupBuiltIn(tt.kind)
			if !result.Found() {
				t.Fatalf("expected %q to be found", tt.kind)
			}
			if result.builtIn.Group != "apps" {
				t.Errorf("expected group=apps, got %q", result.builtIn.Group)
			}
			if result.builtIn.Version != "v1" {
				t.Errorf("expected version=v1, got %q", result.builtIn.Version)
			}
			if result.builtIn.Plural != tt.plural {
				t.Errorf("expected plural=%q, got %q", tt.plural, result.builtIn.Plural)
			}
		})
	}
}

func TestLookupBuiltIn_CaseInsensitive(t *testing.T) {
	variants := []string{"Deployment", "deployment", "DEPLOYMENT", "dEpLoyMeNt"}
	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			result := LookupBuiltIn(v)
			if !result.Found() {
				t.Errorf("expected %q to resolve to Deployment", v)
			}
			if result.Kind() != "Deployment" {
				t.Errorf("expected canonical Kind=Deployment, got %q", result.Kind())
			}
		})
	}
}

func TestLookupBuiltIn_NotFound(t *testing.T) {
	unknowns := []string{"Website", "MyApp", "CustomCRD", ""}
	for _, kind := range unknowns {
		t.Run(kind, func(t *testing.T) {
			result := LookupBuiltIn(kind)
			if result.Found() {
				t.Errorf("expected %q to NOT be found in built-in registry", kind)
			}
		})
	}
}

func TestLookupBuiltIn_CanonicalKindName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"horizontalpodautoscaler", "HorizontalPodAutoscaler"},
		{"persistentvolumeclaim", "PersistentVolumeClaim"},
		{"clusterrolebinding", "ClusterRoleBinding"},
		{"customresourcedefinition", "CustomResourceDefinition"},
		{"poddisruptionbudget", "PodDisruptionBudget"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := LookupBuiltIn(tt.input)
			if !result.Found() {
				t.Fatalf("expected %q to be found", tt.input)
			}
			if result.Kind() != tt.expected {
				t.Errorf("canonical name: expected %q, got %q", tt.expected, result.Kind())
			}
		})
	}
}

func TestAllBuiltInKinds_Sorted(t *testing.T) {
	kinds := AllBuiltInKinds()

	if len(kinds) == 0 {
		t.Fatal("expected non-empty built-in kind list")
	}

	for i := 1; i < len(kinds); i++ {
		if kinds[i] < kinds[i-1] {
			t.Errorf("kinds not sorted: %q comes before %q", kinds[i], kinds[i-1])
		}
	}
}

func TestAllBuiltInKinds_NoInternalAliases(t *testing.T) {
	kinds := AllBuiltInKinds()
	for _, k := range kinds {
		if strings.Contains(k, "_") {
			t.Errorf("internal alias leaked into AllBuiltInKinds: %q", k)
		}
	}
}
