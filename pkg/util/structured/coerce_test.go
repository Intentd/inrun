package structured

import (
	"testing"
)

func noopEval(s string) (string, bool) { return s, true }

func TestResolveArgsMap_StaticPassThrough(t *testing.T) {
	raw := map[string]interface{}{
		"region":   "eu-west-1",
		"replicas": 3,
		"enabled":  true,
	}
	got := ResolveArgsMap(raw, noopEval)
	if got["region"] != "eu-west-1" {
		t.Errorf("region = %v, want eu-west-1", got["region"])
	}
	if got["replicas"] != 3 {
		t.Errorf("replicas = %v, want 3", got["replicas"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled = %v, want true", got["enabled"])
	}
}

func TestResolveArgsMap_BoolCoercion(t *testing.T) {
	raw := map[string]interface{}{"flag": "{{ .spec.flag }}"}
	got := ResolveArgsMap(raw, func(string) (string, bool) { return "true", true })
	if got["flag"] != true {
		t.Errorf("flag = %v (%T), want bool(true)", got["flag"], got["flag"])
	}
}

func TestResolveArgsMap_IntCoercion(t *testing.T) {
	raw := map[string]interface{}{"replicas": "{{ .spec.replicas }}"}
	got := ResolveArgsMap(raw, func(string) (string, bool) { return "3", true })
	if got["replicas"] != float64(3) {
		t.Errorf("replicas = %v (%T), want float64(3)", got["replicas"], got["replicas"])
	}
}

func TestResolveArgsMap_StringRemainsString(t *testing.T) {
	raw := map[string]interface{}{"env": "{{ .spec.env }}"}
	got := ResolveArgsMap(raw, func(string) (string, bool) { return "production", true })
	if got["env"] != "production" {
		t.Errorf("env = %v, want production", got["env"])
	}
}

func TestResolveArgsMap_NestedMap(t *testing.T) {
	raw := map[string]interface{}{
		"featureFlags": map[string]interface{}{
			"newCheckout": "{{ .spec.newCheckout }}",
			"legacy":      false,
		},
	}
	got := ResolveArgsMap(raw, func(string) (string, bool) { return "true", true })
	flags, ok := got["featureFlags"].(map[string]interface{})
	if !ok {
		t.Fatalf("featureFlags is not a map: %T", got["featureFlags"])
	}
	if flags["newCheckout"] != true {
		t.Errorf("newCheckout = %v (%T), want bool(true)", flags["newCheckout"], flags["newCheckout"])
	}
	if flags["legacy"] != false {
		t.Errorf("legacy = %v, want false", flags["legacy"])
	}
}

func TestResolveArgsMap_EvalFailureFallsBack(t *testing.T) {
	raw := map[string]interface{}{"val": "{{ .spec.val }}"}
	got := ResolveArgsMap(raw, func(string) (string, bool) { return "", false })
	if got["val"] != "{{ .spec.val }}" {
		t.Errorf("val = %v, want original template string", got["val"])
	}
}

func TestResolveArgsMap_NonTemplateStringUnchanged(t *testing.T) {
	raw := map[string]interface{}{"region": "us-east-1"}
	called := false
	ResolveArgsMap(raw, func(string) (string, bool) { called = true; return "", true })
	if called {
		t.Error("eval should not be called for non-template strings")
	}
}
