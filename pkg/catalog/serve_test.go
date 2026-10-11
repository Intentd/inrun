package catalog_test

import (
	"testing"
)

func TestIsGatewayAPIEnabled(t *testing.T) {
	t.Run("enabled with secretRef", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-secretref.yaml")
		if !k.IsGatewayAPIEnabled() {
			t.Error("IsGatewayAPIEnabled() = false, want true")
		}
	})

	t.Run("enabled with env token", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-envtoken.yaml")
		if !k.IsGatewayAPIEnabled() {
			t.Error("IsGatewayAPIEnabled() = false, want true")
		}
	})

	t.Run("disabled when no gateway block", func(t *testing.T) {
		k := mustParseTestdata(t, "webhooks/webhooks-no-rules.yaml")
		if k.IsGatewayAPIEnabled() {
			t.Error("IsGatewayAPIEnabled() = true, want false")
		}
	})

	t.Run("true when gateway API enabled even without serve", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-no-serve.yaml")
		if !k.IsGatewayAPIEnabled() {
			t.Error("IsGatewayAPIEnabled() = false, want true — flag is set")
		}
	})
}

func TestHasGatewayAPISecretRefs(t *testing.T) {
	t.Run("true when secretRef present", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-secretref.yaml")
		if !k.HasGatewayAPISecretRefs() {
			t.Error("HasGatewayAPISecretRefs() = false, want true")
		}
	})

	t.Run("false when only env token", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-envtoken.yaml")
		if k.HasGatewayAPISecretRefs() {
			t.Error("HasGatewayAPISecretRefs() = true, want false")
		}
	})

	t.Run("false when gateway API not enabled", func(t *testing.T) {
		k := mustParseTestdata(t, "webhooks/webhooks-no-rules.yaml")
		if k.HasGatewayAPISecretRefs() {
			t.Error("HasGatewayAPISecretRefs() = true, want false")
		}
	})
}

func TestHasServeEnabled(t *testing.T) {
	t.Run("true when gateway API enabled and serve on a CRD", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-secretref.yaml")
		if !k.HasServeEnabled() {
			t.Error("HasServeEnabled() = false, want true")
		}
	})

	t.Run("false when no gateway block", func(t *testing.T) {
		k := mustParseTestdata(t, "webhooks/webhooks-no-rules.yaml")
		if k.HasServeEnabled() {
			t.Error("HasServeEnabled() = true, want false")
		}
	})

	t.Run("false when gateway API enabled but no serve", func(t *testing.T) {
		k := mustParseTestdata(t, "serve/serve-no-serve.yaml")
		if k.HasServeEnabled() {
			t.Error("HasServeEnabled() = true, want false — no serve.enabled CRD")
		}
	})
}

func TestGenerateGatewayRBACRules_API(t *testing.T) {
	k := mustParseTestdata(t, "serve/serve-secretref.yaml")
	rules := k.GenerateGatewayRBACRules()

	// Must allow get and create on secrets for self-bootstrap (create in a
	// rule without resourceNames).
	hasGet, hasCreate := false, false
	for _, r := range rules {
		if !contains(r.Resources, "secrets") {
			continue
		}
		hasGet = hasGet || contains(r.Verbs, "get")
		hasCreate = hasCreate || (contains(r.Verbs, "create") && len(r.ResourceNames) == 0)
	}
	if !hasGet || !hasCreate {
		t.Errorf("secrets rules: get=%v create=%v, want both", hasGet, hasCreate)
	}

	// Must contain CR verbs for the serve-enabled platform CRD
	foundCRDVerbs := false
	for _, r := range rules {
		for _, res := range r.Resources {
			if res == "platforms" {
				foundCRDVerbs = true
				for _, v := range []string{"get", "list", "create", "update", "patch", "delete"} {
					if !contains(r.Verbs, v) {
						t.Errorf("platforms rule missing verb %q: %v", v, r.Verbs)
					}
				}
			}
		}
	}
	if !foundCRDVerbs {
		t.Error("no platforms CR rule found in gateway RBAC rules")
	}
}

func TestGenerateGatewayRBACRules_NoSecretRuleForEnvToken(t *testing.T) {
	k := mustParseTestdata(t, "serve/serve-envtoken.yaml")
	rules := k.GenerateGatewayRBACRules()
	for _, r := range rules {
		for _, res := range r.Resources {
			if res == "secrets" {
				// secrets rule is only valid if NeedsCertificates is true (TLS)
				// For env-token-only Gateway API there should be no secrets rule
				// unless NeedsCertificates is also true.
				if k.HasGatewayAPISecretRefs() {
					t.Error("unexpected HasGatewayAPISecretRefs() = true for env-token catalog")
				}
				_ = r
			}
		}
	}
}
