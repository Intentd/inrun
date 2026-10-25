package blueprint_test

import (
	"reflect"
	"testing"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/types"
	rbacv1 "k8s.io/api/rbac/v1"
)

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func mustParseTestdata(t *testing.T, name string) *blueprint.Blueprint {
	t.Helper()
	bp, err := blueprint.ParseFile("testdata/" + name)
	if err != nil {
		t.Fatalf("ParseFile testdata/%s: %v", name, err)
	}
	return bp
}

func TestWebhookResources_NoRules(t *testing.T) {
	bp := mustParseTestdata(t, "webhooks/webhooks-no-rules.yaml")
	if got := bp.WebhookResources(); len(got) != 0 {
		t.Fatalf("expected no webhook resources, got %v", got)
	}
}

func TestWebhookResources_EnabledNoRules(t *testing.T) {
	bp := mustParseTestdata(t, "webhooks/webhooks-no-rules.yaml")
	if got := bp.WebhookResources(); len(got) != 0 {
		t.Fatalf("expected no webhook resources with admission enabled but no rules, got %v", got)
	}
}

func TestWebhookResources_ValidationOnly(t *testing.T) {
	bp := mustParseTestdata(t, "webhooks/webhooks-validation.yaml")
	got := bp.WebhookResources()
	if !contains(got, "validatingwebhookconfigurations") {
		t.Fatalf("expected validatingwebhookconfigurations, got %v", got)
	}
	if contains(got, "mutatingwebhookconfigurations") {
		t.Fatalf("did not expect mutatingwebhookconfigurations for validation-only: %v", got)
	}
}

func TestWebhookResources_MutationOnly(t *testing.T) {
	bp := mustParseTestdata(t, "webhooks/webhooks-mutation.yaml")
	got := bp.WebhookResources()
	if !contains(got, "mutatingwebhookconfigurations") {
		t.Fatalf("expected mutatingwebhookconfigurations, got %v", got)
	}
	if contains(got, "validatingwebhookconfigurations") {
		t.Fatalf("did not expect validatingwebhookconfigurations for mutation-only: %v", got)
	}
}

func TestWebhookResources_BothValidationAndMutation(t *testing.T) {
	bp := mustParseTestdata(t, "webhooks/webhooks-both.yaml")
	got := bp.WebhookResources()
	if !contains(got, "validatingwebhookconfigurations") || !contains(got, "mutatingwebhookconfigurations") {
		t.Fatalf("expected both webhook resources, got %v", got)
	}
}

func TestGenerateRBACRules_CustomResources(t *testing.T) {
	bp := mustParseTestdata(t, "rbac/custom-rbac.yaml")
	rules := bp.GenerateRBACRules()

	want := map[string]string{
		"issuers":      "cert-manager.io",
		"certificates": "cert-manager.io",
	}
	for resource, group := range want {
		found := false
		for _, r := range rules {
			if contains(r.APIGroups, group) && contains(r.Resources, resource) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected RBAC rule for %s/%s — not found in generated rules", group, resource)
		}
	}
}

func TestGenerateRBACRules_CustomResources_Dedup(t *testing.T) {
	bp := mustParseTestdata(t, "rbac/custom-rbac-dedup.yaml")
	rules := bp.GenerateRBACRules()

	count := 0
	for _, r := range rules {
		if contains(r.APIGroups, "cert-manager.io") && contains(r.Resources, "certificates") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 RBAC rule for cert-manager.io/certificates, got %d", count)
	}
}

// ── helpers for cluster RBAC tests ───────────────────────────────────────────

func newWidgetCRD(cluster string) types.CRDEntry {
	var clusters []string
	if cluster != "" {
		clusters = []string{cluster}
	}
	return types.CRDEntry{
		APITypes: types.APITypes{
			Group:   "example.io",
			Version: "v1alpha1",
			Kind:    "Widget",
			Plural:  "widgets",
		},
		Serve: &types.ServeConfig{
			Enabled:  true,
			Clusters: clusters,
		},
	}
}

func gatewayWithClusters(names ...string) *types.GatewayConfig {
	entries := make(map[string]types.GatewayClusterConfig, len(names))
	for _, n := range names {
		entries[n] = types.GatewayClusterConfig{Endpoint: "https://" + n + ".internal:6443"}
	}
	return &types.GatewayConfig{
		Enabled: true,
		API:     &types.GatewayAPIConfig{Enabled: true},
		Clusters: &types.GatewayClustersConfig{
			Entries: entries,
		},
	}
}

func hasRuleForGroup(rules []rbacv1.PolicyRule, group string) bool {
	for _, r := range rules {
		for _, g := range r.APIGroups {
			if g == group {
				return true
			}
		}
	}
	return false
}

// ── GenerateGatewayRBACRules — cluster routing filter ────────────────────────

func TestGenerateGatewayRBACRules_LocalCRDIncluded(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"widget": newWidgetCRD(""), // no cluster = local
	})
	bp.Gateway = gatewayWithClusters("prod")

	rules := bp.GenerateGatewayRBACRules()
	found := false
	for _, r := range rules {
		if contains(r.APIGroups, "example.io") && contains(r.Resources, "widgets") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected local-routed CRD to appear in GenerateGatewayRBACRules, but it didn't")
	}
}

func TestGenerateGatewayRBACRules_RemoteCRDExcluded(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"widget": newWidgetCRD("prod"), // static remote cluster
	})
	bp.Gateway = gatewayWithClusters("prod")

	rules := bp.GenerateGatewayRBACRules()
	for _, r := range rules {
		if contains(r.APIGroups, "example.io") && contains(r.Resources, "widgets") {
			t.Error("remote-routed CRD should NOT appear in GenerateGatewayRBACRules, but it did")
		}
	}
}

// ── GenerateGatewayClusterRBACRules ──────────────────────────────────────────

func TestGenerateGatewayClusterRBACRules_NoClusters(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"widget": newWidgetCRD("prod"),
	})
	// No Gateway configured.
	rules, warns := bp.GenerateGatewayClusterRBACRules()
	if rules != nil || warns != nil {
		t.Errorf("expected nil, nil when no clusters registered; got rules=%v warns=%v", rules, warns)
	}
}

func TestGenerateGatewayClusterRBACRules_StaticCluster(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"widget": newWidgetCRD("prod"),
	})
	bp.Gateway = gatewayWithClusters("prod", "staging")

	clusterRules, warns := bp.GenerateGatewayClusterRBACRules()

	if len(warns) != 0 {
		t.Errorf("expected no template warnings, got %v", warns)
	}

	prodRules, ok := clusterRules["prod"]
	if !ok || len(prodRules) == 0 {
		t.Error("expected rules for cluster 'prod', got none")
	}

	found := false
	for _, r := range prodRules {
		if contains(r.APIGroups, "example.io") && contains(r.Resources, "widgets") {
			found = true
			break
		}
	}
	if !found {
		t.Error("widget rule missing from prod cluster rules")
	}

	// staging has no CRDs routed to it — should be absent from the map.
	if _, ok := clusterRules["staging"]; ok {
		t.Error("staging should not appear in cluster rules (no CRDs route there)")
	}
}

func TestGenerateGatewayClusterRBACRules_LocalCRDNotInClusterMap(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"widget": newWidgetCRD(""), // local
	})
	bp.Gateway = gatewayWithClusters("prod")

	clusterRules, _ := bp.GenerateGatewayClusterRBACRules()
	if len(clusterRules) != 0 {
		t.Errorf("local-routed CRD should produce no cluster RBAC entries, got %v", clusterRules)
	}
}

func TestGenerateGatewayClusterRBACRules_TargetOverride(t *testing.T) {
	// serve.clusters: [staging], target.prod-only.clusters: [prod]
	widget := types.CRDEntry{
		APITypes: types.APITypes{Group: "example.io", Version: "v1alpha1", Kind: "Widget", Plural: "widgets"},
		Serve: &types.ServeConfig{
			Enabled:  true,
			Clusters: []string{"staging", "prod"},
			Target: types.ServeTargetValue{
				Entries: map[string]*types.ServeTargetConfig{
					"prod-only": {Primary: true, Clusters: []string{"prod"}},
				},
			},
		},
	}
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"widget": widget})
	bp.Gateway = gatewayWithClusters("prod", "staging")

	clusterRules, warns := bp.GenerateGatewayClusterRBACRules()

	if len(warns) != 0 {
		t.Errorf("expected no template warnings, got %v", warns)
	}
	for _, cluster := range []string{"prod", "staging"} {
		rules, ok := clusterRules[cluster]
		if !ok || len(rules) == 0 {
			t.Errorf("expected rules for cluster %q, got none", cluster)
			continue
		}
		found := false
		for _, r := range rules {
			if contains(r.APIGroups, "example.io") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("widget rule missing from cluster %q", cluster)
		}
	}
}

func TestGenerateGatewayClusterRBACRules_TemplateCluster(t *testing.T) {
	widget := types.CRDEntry{
		APITypes: types.APITypes{Group: "example.io", Version: "v1alpha1", Kind: "Widget", Plural: "widgets"},
		Serve: &types.ServeConfig{
			Enabled:  true,
			Clusters: []string{`{{ if eq .request.env "prod" }}prod{{ else }}staging{{ end }}`},
		},
	}
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"widget": widget})
	bp.Gateway = gatewayWithClusters("prod", "staging")

	clusterRules, warns := bp.GenerateGatewayClusterRBACRules()

	if len(warns) == 0 {
		t.Error("expected a template warning for Widget, got none")
	}
	if !contains(warns, "Widget") {
		t.Errorf("expected 'Widget' in template warnings, got %v", warns)
	}

	// Template-routed CRD must appear in ALL clusters.
	for _, cluster := range []string{"prod", "staging"} {
		rules, ok := clusterRules[cluster]
		if !ok || len(rules) == 0 {
			t.Errorf("template-routed CRD should appear in cluster %q, but it didn't", cluster)
		}
	}
}

// ── secretNamesFrom / scoped secrets rules ───────────────────────────────────

func hasSecretRuleScoped(rules []rbacv1.PolicyRule, names ...string) bool {
	for _, r := range rules {
		if !contains(r.Resources, "secrets") || !contains(r.APIGroups, "") {
			continue
		}
		if len(r.ResourceNames) == 0 {
			continue
		}
		match := true
		for _, n := range names {
			if !contains(r.ResourceNames, n) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func hasSecretRuleUnscoped(rules []rbacv1.PolicyRule) bool {
	for _, r := range rules {
		if contains(r.Resources, "secrets") && contains(r.APIGroups, "") && len(r.ResourceNames) == 0 {
			return true
		}
	}
	return false
}

func remoteCRD(endpoint, secretName string, resources []domain.ManagedResource) types.CRDEntry {
	auth := (*types.ExternalAuth)(nil)
	if secretName != "" {
		auth = &types.ExternalAuth{
			SecretRef: &types.APISecretRef{Name: secretName, Key: "token", Namespace: "inrun"},
		}
	}
	return types.CRDEntry{
		APITypes: types.APITypes{Group: "example.io", Version: "v1alpha1", Kind: "Widget", Plural: "widgets"},
		OperatorBox: &types.OperatorBoxConfig{
			Reconcile: &types.ReconcileConfig{
				ManagedResources: resources,
				Remote: &types.RemoteReconcilerDeclaration{
					Endpoint: endpoint,
					Auth:     auth,
				},
			},
		},
	}
}

// Remote managedResources feed into all three RBAC generators.
func TestGenerateRBACRules_RemoteManagedResources(t *testing.T) {
	crd := remoteCRD("https://svc/reconcile", "", []domain.ManagedResource{
		{Group: "apps", Version: "v1", Plural: "deployments"},
	})
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"widget": crd})

	for _, tc := range []struct {
		name  string
		rules []rbacv1.PolicyRule
	}{
		{"GenerateRBACRules", bp.GenerateRBACRules()},
		{"GenerateRuntimeRBACRules", bp.GenerateRuntimeRBACRules()},
	} {
		found := false
		for _, r := range tc.rules {
			if contains(r.APIGroups, "apps") && contains(r.Resources, "deployments") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: expected deployments rule from remote.managedResources — not found", tc.name)
		}
	}
}

// Static remote auth.secretRef → scoped rule.
func TestGenerateRuntimeRBACRules_RemoteSecretRef_Static(t *testing.T) {
	crd := remoteCRD("https://svc/reconcile", "my-token", nil)
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"widget": crd})
	rules := bp.GenerateRuntimeRBACRules()

	if !hasSecretRuleScoped(rules, "my-token") {
		t.Error("expected scoped secrets rule for 'my-token'")
	}
	if hasSecretRuleUnscoped(rules) {
		t.Error("did not expect unscoped secrets rule for all-static refs")
	}
}

// Template remote auth.secretRef → unscoped rule only.
func TestGenerateRuntimeRBACRules_RemoteSecretRef_Template(t *testing.T) {
	crd := remoteCRD("https://svc/reconcile", `{{ .request.token }}`, nil)
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"widget": crd})
	rules := bp.GenerateRuntimeRBACRules()

	if !hasSecretRuleUnscoped(rules) {
		t.Error("expected unscoped secrets rule for template ref")
	}
}

// Mixed: one static CRD + one template CRD → both a scoped rule and an unscoped rule.
func TestGenerateRuntimeRBACRules_RemoteSecretRef_Mixed(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"static-crd":   remoteCRD("https://svc/reconcile", "static-token", nil),
		"template-crd": remoteCRD("https://svc2/reconcile", `{{ .request.token }}`, nil),
	})
	rules := bp.GenerateRuntimeRBACRules()

	if !hasSecretRuleScoped(rules, "static-token") {
		t.Error("expected scoped secrets rule for 'static-token' even when template ref exists")
	}
	if !hasSecretRuleUnscoped(rules) {
		t.Error("expected unscoped secrets rule for template ref")
	}
}

// Gateway API token with static secretRef → scoped get/create rule.
func TestGenerateGatewayRBACRules_GatewaySecretRef_Static(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(nil)
	bp.Gateway = &types.GatewayConfig{
		Enabled: true,
		API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth: types.APIAuth{
				Tokens: []types.APIToken{
					{SecretRef: &types.APISecretRef{Name: "gw-token", Key: "token"}},
				},
			},
		},
	}

	rules := bp.GenerateGatewayRBACRules()

	// get is scoped to the name; create cannot be (Kubernetes never
	// authorizes create by resource name), so it is a rule of its own.
	hasScopedGet, hasCreate := false, false
	for _, r := range rules {
		if !contains(r.Resources, "secrets") {
			continue
		}
		if contains(r.Verbs, "get") && contains(r.ResourceNames, "gw-token") {
			hasScopedGet = true
		}
		if contains(r.Verbs, "create") {
			if len(r.ResourceNames) > 0 {
				t.Errorf("create must not be scoped by resourceNames: %v", r.ResourceNames)
			}
			hasCreate = true
		}
	}
	if !hasScopedGet {
		t.Error("expected secrets get scoped to gw-token")
	}
	if !hasCreate {
		t.Error("expected secrets create for token bootstrap")
	}
}

// Gateway API token with template secretRef → unscoped get/create rule.
func TestGenerateGatewayRBACRules_GatewaySecretRef_Template(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(nil)
	bp.Gateway = &types.GatewayConfig{
		Enabled: true,
		API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth: types.APIAuth{
				Tokens: []types.APIToken{
					{SecretRef: &types.APISecretRef{Name: `{{ .request.tokenName }}`, Key: "token"}},
				},
			},
		},
	}

	rules := bp.GenerateGatewayRBACRules()

	hasUnscoped := false
	for _, r := range rules {
		if contains(r.Resources, "secrets") && contains(r.Verbs, "create") && len(r.ResourceNames) == 0 {
			hasUnscoped = true
			break
		}
	}
	if !hasUnscoped {
		t.Error("expected unscoped secrets get/create rule for gateway template secretRef")
	}
}

// Gateway API: one static + one template → both rules emitted.
func TestGenerateGatewayRBACRules_GatewaySecretRef_Mixed(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(nil)
	bp.Gateway = &types.GatewayConfig{
		Enabled: true,
		API: &types.GatewayAPIConfig{
			Enabled: true,
			Auth: types.APIAuth{
				Tokens: []types.APIToken{
					{SecretRef: &types.APISecretRef{Name: "static-gw", Key: "token"}},
					{SecretRef: &types.APISecretRef{Name: `{{ .request.dynamic }}`, Key: "token"}},
				},
			},
		},
	}

	rules := bp.GenerateGatewayRBACRules()

	hasScoped, hasUnscoped := false, false
	for _, r := range rules {
		if !contains(r.Resources, "secrets") {
			continue
		}
		if contains(r.Verbs, "get") && contains(r.ResourceNames, "static-gw") {
			hasScoped = true
		}
		if contains(r.Verbs, "create") && len(r.ResourceNames) == 0 {
			hasUnscoped = true
		}
	}
	if !hasScoped {
		t.Error("expected scoped secrets rule for 'static-gw' even when template token exists")
	}
	if !hasUnscoped {
		t.Error("expected unscoped secrets rule for template token")
	}
}

func TestGenerateGatewayClusterRBACRules_MultiCRD(t *testing.T) {
	crds := map[string]types.CRDEntry{
		"widget": newWidgetCRD("prod"),
		"gadget": {
			APITypes: types.APITypes{Group: "example.io", Version: "v1alpha1", Kind: "Gadget", Plural: "gadgets"},
			Serve:    &types.ServeConfig{Enabled: true, Clusters: []string{"staging"}},
		},
		"gizmo": {
			APITypes: types.APITypes{Group: "example.io", Version: "v1alpha1", Kind: "Gizmo", Plural: "gizmos"},
			Serve:    &types.ServeConfig{Enabled: true}, // local
		},
	}
	bp := blueprint.NewBlueprintForTest(crds)
	bp.Gateway = gatewayWithClusters("prod", "staging")

	clusterRules, warns := bp.GenerateGatewayClusterRBACRules()

	if len(warns) != 0 {
		t.Errorf("expected no template warnings, got %v", warns)
	}

	hasResource := func(rules []rbacv1.PolicyRule, res string) bool {
		for _, r := range rules {
			if contains(r.Resources, res) {
				return true
			}
		}
		return false
	}

	if !hasResource(clusterRules["prod"], "widgets") {
		t.Error("expected 'widgets' in prod cluster rules")
	}
	if hasResource(clusterRules["prod"], "gadgets") {
		t.Error("'gadgets' should not appear in prod cluster rules")
	}

	if !hasResource(clusterRules["staging"], "gadgets") {
		t.Error("expected 'gadgets' in staging cluster rules")
	}
	if hasResource(clusterRules["staging"], "widgets") {
		t.Error("'widgets' should not appear in staging cluster rules")
	}

	// gizmo is local — not in any cluster map.
	for cluster, rules := range clusterRules {
		if hasResource(rules, "gizmos") {
			t.Errorf("local-routed 'gizmos' should not appear in cluster %q rules", cluster)
		}
	}
}

// ── migrate.to ───────────────────────────────────────────────────────────────

func migrateRules(t *testing.T, prune bool) []rbacv1.PolicyRule {
	t.Helper()
	crd := newWidgetCRD("")
	crd.Serve = nil
	crd.Migrate = &types.CRMigrate{
		To:    types.CRMigrateTarget{APIVersion: "next.io/v2", Kind: "Gadget", Plural: "gadgets"},
		Prune: prune,
	}
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{"widget": crd})
	bp.Gateway = gatewayWithClusters()
	var out []rbacv1.PolicyRule
	for _, r := range bp.GenerateGatewayRBACRules() {
		if contains(r.Resources, "gadgets") || contains(r.Resources, "widgets") {
			out = append(out, r)
		}
	}
	return out
}

func TestGenerateGatewayRBACRules_MigrateTo(t *testing.T) {
	want := []rbacv1.PolicyRule{
		{APIGroups: []string{"next.io"}, Resources: []string{"gadgets"}, Verbs: []string{"get", "create", "patch"}},
		{APIGroups: []string{"example.io"}, Resources: []string{"widgets"}, Verbs: []string{"get", "list"}},
	}
	if got := migrateRules(t, false); !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %+v\nwant %+v", got, want)
	}
}

func TestGenerateGatewayRBACRules_MigrateToPruneDeletesSource(t *testing.T) {
	got := migrateRules(t, true)
	if len(got) != 2 || !contains(got[1].Verbs, "delete") {
		t.Errorf("source rule must allow delete with prune: %+v", got)
	}
	if contains(got[0].Verbs, "delete") {
		t.Errorf("destination rule must not allow delete: %+v", got[0])
	}
}
