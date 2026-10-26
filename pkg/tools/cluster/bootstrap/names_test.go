package bootstrap

import "testing"

func TestSAName_Default(t *testing.T) {
	e := ClusterEntry{Name: "staging", Context: "kind-staging"}
	if got := SAName(e); got != DefaultSAName {
		t.Errorf("expected %q, got %q", DefaultSAName, got)
	}
}

func TestSAName_Override(t *testing.T) {
	e := ClusterEntry{Name: "staging", Context: "kind-staging", SAName: "argocd-inrun-generated"}
	if got := SAName(e); got != "argocd-inrun-generated" {
		t.Errorf("expected %q, got %q", "argocd-inrun-generated", got)
	}
}

func TestClusterRoleName_MatchesSAName(t *testing.T) {
	e := ClusterEntry{Name: "prod", Context: "kind-prod", SAName: "custom-sa"}
	if ClusterRoleName(e) != SAName(e) {
		t.Error("ClusterRoleName should match SAName")
	}
}

func TestCRBName_MatchesSAName(t *testing.T) {
	e := ClusterEntry{Name: "prod", Context: "kind-prod"}
	if CRBName(e) != SAName(e) {
		t.Error("CRBName should match SAName")
	}
}

func TestTokenSecretName(t *testing.T) {
	e := ClusterEntry{Name: "staging", Context: "kind-staging"}
	if got := TokenSecretName(e); got != DefaultSAName+"-token" {
		t.Errorf("expected %q, got %q", DefaultSAName+"-token", got)
	}
}

func TestTokenSecretName_Override(t *testing.T) {
	e := ClusterEntry{Name: "staging", Context: "kind-staging", SAName: "argocd-inrun-generated"}
	if got := TokenSecretName(e); got != "argocd-inrun-generated-token" {
		t.Errorf("expected %q, got %q", "argocd-inrun-generated-token", got)
	}
}

func TestGatewaySecretName(t *testing.T) {
	e := ClusterEntry{Name: "staging"}
	if got := GatewaySecretName(e); got != "inrun-staging" {
		t.Errorf("expected %q, got %q", "inrun-staging", got)
	}
}

func TestGatewaySecretName_Prod(t *testing.T) {
	e := ClusterEntry{Name: "prod"}
	if got := GatewaySecretName(e); got != "inrun-prod" {
		t.Errorf("expected %q, got %q", "inrun-prod", got)
	}
}
