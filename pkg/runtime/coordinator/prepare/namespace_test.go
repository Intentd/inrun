package prepare_test

import (
	"context"
	"testing"

	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func guardObj(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "demo.inrun.dev/v1",
			"kind":       "Website",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func TestCheckNamespace_NoRestrictions_AllAllowed(t *testing.T) {
	obj := guardObj("default", "my-site")
	result := prepare.CheckNamespace(context.Background(), obj, "default", nil, nil, "Website")
	if !result.Allowed {
		t.Error("empty restricted + empty allowed should allow all namespaces")
	}
}

func TestCheckNamespace_ExactMatch_Restricted(t *testing.T) {
	obj := guardObj("default", "my-site")
	restricted := types.RestrictedNamespaces{"kube-system", "cert-manager"}
	result := prepare.CheckNamespace(context.Background(), obj, "kube-system", restricted, nil, "Website")
	if result.Allowed {
		t.Error("kube-system is restricted — should be blocked")
	}
	if result.Reason != "restricted" {
		t.Errorf("expected reason 'restricted', got %q", result.Reason)
	}
	if result.Pattern == "" {
		t.Error("matched pattern should be set when restricted")
	}
}

func TestCheckNamespace_WildcardPrefix_Restricted(t *testing.T) {
	obj := guardObj("default", "my-site")
	restricted := types.RestrictedNamespaces{"kube-*"}
	result := prepare.CheckNamespace(context.Background(), obj, "kube-public", restricted, nil, "Website")
	if result.Allowed {
		t.Error("kube-public should be blocked by kube-*")
	}
	if result.Pattern != "kube-*" {
		t.Errorf("expected pattern kube-*, got %q", result.Pattern)
	}
}

func TestCheckNamespace_NotRestricted_Allowed(t *testing.T) {
	obj := guardObj("default", "my-site")
	restricted := types.RestrictedNamespaces{"kube-system", "cert-manager"}
	result := prepare.CheckNamespace(context.Background(), obj, "production", restricted, nil, "Website")
	if !result.Allowed {
		t.Error("production is not restricted — should be allowed")
	}
	if result.Pattern != "" {
		t.Errorf("pattern should be empty when allowed, got %q", result.Pattern)
	}
}

func TestCheckNamespace_AllowedList_AllowsMatch(t *testing.T) {
	obj := guardObj("default", "my-site")
	allowed := types.AllowedNamespaces{"team-*", "apps"}
	result := prepare.CheckNamespace(context.Background(), obj, "team-alpha", nil, allowed, "Website")
	if !result.Allowed {
		t.Error("team-alpha matches team-* — should be allowed")
	}
}

func TestCheckNamespace_AllowedList_BlocksNonMatch(t *testing.T) {
	obj := guardObj("default", "my-site")
	allowed := types.AllowedNamespaces{"team-*", "apps"}
	result := prepare.CheckNamespace(context.Background(), obj, "default", nil, allowed, "Website")
	if result.Allowed {
		t.Error("default is not in allowed list — should be blocked")
	}
	if result.Reason != "not-allowed" {
		t.Errorf("expected reason 'not-allowed', got %q", result.Reason)
	}
}

func TestCheckNamespace_RestrictedOverridesAllowed(t *testing.T) {
	obj := guardObj("default", "my-site")
	restricted := types.RestrictedNamespaces{"prod-*"}
	allowed := types.AllowedNamespaces{"prod-team", "prod-alpha"}
	result := prepare.CheckNamespace(context.Background(), obj, "prod-team", restricted, allowed, "Website")
	if result.Allowed {
		t.Error("restricted must override allowed — should be blocked")
	}
	if result.Reason != "restricted" {
		t.Errorf("expected reason 'restricted', got %q", result.Reason)
	}
}

func TestNamespaceGuardResult_EventMessage_Restricted(t *testing.T) {
	obj := guardObj("default", "my-site")
	restricted := types.RestrictedNamespaces{"kube-system"}
	result := prepare.CheckNamespace(context.Background(), obj, "kube-system", restricted, nil, "Website")
	msg := result.EventMessage("Deployment", "my-app")
	if msg == "" {
		t.Error("EventMessage should return a non-empty string")
	}
	if result.Reason != "restricted" {
		t.Errorf("expected restricted reason, got %q", result.Reason)
	}
}

func TestNamespaceGuardResult_EventMessage_NotAllowed(t *testing.T) {
	obj := guardObj("default", "my-site")
	allowed := types.AllowedNamespaces{"team-*"}
	result := prepare.CheckNamespace(context.Background(), obj, "default", nil, allowed, "Website")
	if result.Allowed {
		t.Error("default is not allowed — should be blocked")
	}
	msg := result.EventMessage("Service", "backend")
	if msg == "" {
		t.Error("EventMessage should return a non-empty string")
	}
	if result.Reason != "not-allowed" {
		t.Errorf("expected not-allowed reason, got %q", result.Reason)
	}
}
