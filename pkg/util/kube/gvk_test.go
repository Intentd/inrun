package kube

import (
	"strings"
	"testing"
)

func TestSetGroupVersionKind_Format(t *testing.T) {
	gvk := SetGroupVersionKind("apps", "v1", "Deployment")
	s := gvk.String()

	if !strings.Contains(s, "apps") {
		t.Errorf("expected group in output, got %q", s)
	}
	if !strings.Contains(s, "v1") {
		t.Errorf("expected version in output, got %q", s)
	}
	if !strings.Contains(s, "Deployment") {
		t.Errorf("expected kind in output, got %q", s)
	}
}

func TestSetGroupVersionKind_EmptyGroup(t *testing.T) {
	gvk := SetGroupVersionKind("", "v1", "Pod")
	s := gvk.String()
	// Core group is empty string — result must still contain version and kind
	if !strings.Contains(s, "v1") || !strings.Contains(s, "Pod") {
		t.Errorf("expected v1/Pod in output, got %q", s)
	}
}

func TestSetGroupVersionKind_String(t *testing.T) {
	gvk := SetGroupVersionKind("inrun.dev", "v1", "Website")
	if gvk.String() == "" {
		t.Error("String() must return non-empty value")
	}
}

func TestGroupVersionKind_Roundtrip(t *testing.T) {
	g, v, k := "batch", "v1", "Job"
	gvk := SetGroupVersionKind(g, v, k)
	s := gvk.String()
	if !strings.Contains(s, g) || !strings.Contains(s, v) || !strings.Contains(s, k) {
		t.Errorf("roundtrip failed: %q missing group/version/kind", s)
	}
}

func TestGVRKeyAndGVKKey(t *testing.T) {
	if got := GVRKey("apps", "v1", "deployments"); got != "apps/v1/deployments" {
		t.Errorf("GVRKey = %q", got)
	}
	if got := GVRKey("", "v1", "configmaps"); got != "v1/configmaps" {
		t.Errorf("GVRKey core = %q", got)
	}
	if got := GVKKey("example.io", "v1", "App"); got != "example.io/v1/App" {
		t.Errorf("GVKKey = %q", got)
	}
}

func TestPluralForKindAndGVRFromFields(t *testing.T) {
	for kind, want := range map[string]string{
		"Deployment": "deployments", "NetworkPolicy": "networkpolicies", "Ingress": "ingresses",
		"Policy": "policies", "Endpoints": "endpoints",
	} {
		if got := PluralForKind(kind); got != want {
			t.Errorf("PluralForKind(%q) = %q, want %q", kind, got, want)
		}
	}
	gvr, err := GVRFromFields("db.example.io/v1", "Policy", "")
	if err != nil || gvr.Group != "db.example.io" || gvr.Version != "v1" || gvr.Resource != "policies" {
		t.Errorf("GVRFromFields guessed = %v, %v", gvr, err)
	}
	if gvr, _ := GVRFromFields("db.example.io/v1", "Policy", "Pols"); gvr.Resource != "pols" {
		t.Errorf("GVRFromFields with plural = %v; want the declared plural, lowercased", gvr)
	}
	if gvr, _ := GVRFromFields("v1", "ConfigMap", ""); gvr.Group != "" || gvr.Resource != "configmaps" {
		t.Errorf("core GVR = %v", gvr)
	}
}
