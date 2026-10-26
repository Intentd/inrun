package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestLoadConfig_Valid(t *testing.T) {
	yaml := `
clusters:
  - name: staging
    context: kind-inrun-multi-2
  - name: prod
    context: kind-inrun-multi-3
    sa-namespace: restricted-ns
    sa-name: argocd-inrun-generated
`
	cfg := mustLoadYAML(t, yaml)

	if len(cfg.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(cfg.Clusters))
	}

	s := cfg.Clusters[0]
	if s.Name != "staging" || s.Context != "kind-inrun-multi-2" {
		t.Errorf("unexpected staging entry: %+v", s)
	}
	if s.SANamespace != DefaultSANamespace {
		t.Errorf("expected default sa-namespace %q, got %q", DefaultSANamespace, s.SANamespace)
	}

	p := cfg.Clusters[1]
	if p.SANamespace != "restricted-ns" {
		t.Errorf("expected sa-namespace %q, got %q", "restricted-ns", p.SANamespace)
	}
	if p.SAName != "argocd-inrun-generated" {
		t.Errorf("expected sa-name %q, got %q", "argocd-inrun-generated", p.SAName)
	}
}

func TestLoadConfig_DefaultsApplied(t *testing.T) {
	yaml := `
clusters:
  - name: staging
    context: kind-staging
`
	cfg := mustLoadYAML(t, yaml)
	if cfg.Clusters[0].SANamespace != DefaultSANamespace {
		t.Errorf("default sa-namespace not applied: got %q", cfg.Clusters[0].SANamespace)
	}
}

func TestValidateConfig_MissingName(t *testing.T) {
	cfg := &ConfigFile{Clusters: []ClusterEntry{
		{Context: "kind-staging"},
	}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidateConfig_MissingContext(t *testing.T) {
	cfg := &ConfigFile{Clusters: []ClusterEntry{
		{Name: "staging"},
	}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for missing context")
	}
}

func TestValidateConfig_DuplicateNames(t *testing.T) {
	cfg := &ConfigFile{Clusters: []ClusterEntry{
		{Name: "prod", Context: "kind-prod-1"},
		{Name: "prod", Context: "kind-prod-2"},
	}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestValidateConfig_EmptyClusters(t *testing.T) {
	cfg := &ConfigFile{}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for empty clusters list")
	}
}

func TestValidateConfig_InvalidVerb(t *testing.T) {
	cfg := &ConfigFile{Clusters: []ClusterEntry{
		{
			Name:    "staging",
			Context: "kind-staging",
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "squash"}},
			},
		},
	}}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for unknown verb")
	}
}

func TestValidateConfig_WildcardVerbAllowed(t *testing.T) {
	cfg := &ConfigFile{Clusters: []ClusterEntry{
		{
			Name:        "staging",
			Context:     "kind-staging",
			SANamespace: DefaultSANamespace,
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			},
		},
	}}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("wildcard should be valid: %v", err)
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := &ConfigFile{Clusters: []ClusterEntry{
		{Name: "staging", Context: "kind-staging", SANamespace: DefaultSANamespace},
		{Name: "prod", Context: "kind-prod", SANamespace: DefaultSANamespace},
	}}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// mustLoadYAML writes yaml to a temp file and calls LoadConfig.
func mustLoadYAML(t *testing.T, yaml string) *ConfigFile {
	t.Helper()
	f := filepath.Join(t.TempDir(), "cluster-config.yaml")
	if err := os.WriteFile(f, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}
