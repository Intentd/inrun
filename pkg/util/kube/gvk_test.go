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
