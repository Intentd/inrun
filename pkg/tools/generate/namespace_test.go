package generate_test

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/tools/generate"
	rbacv1 "k8s.io/api/rbac/v1"
)

const testNamespace = "test-namespace"

// minimalExpandedBlueprint returns the smallest valid expanded-blueprint YAML —
// no imports, no CRDs. Used to satisfy ConfigMap / RenderBundle without
// requiring a real merge+expand pipeline.
func minimalExpandedBlueprint() []byte {
	return []byte("apiVersion: inrun.dev/v1\nkind: Blueprint\nmetadata:\n  name: test\nspec:\n  crds: {}\n")
}

// countOccurrences returns how many times sub appears in s.
func countOccurrences(s, sub string) int {
	return strings.Count(s, sub)
}

// ── RBAC standalone ──────────────────────────────────────────────────────────

func TestRBAC_ContainsNamespaceFirst(t *testing.T) {
	output, err := generate.RBAC(nil, nil, testNamespace, "", generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RBAC: %v", err)
	}
	s := string(output)

	nsIdx := strings.Index(s, "kind: Namespace")
	saIdx := strings.Index(s, "kind: ServiceAccount")
	if nsIdx < 0 {
		t.Fatal("RBAC output missing kind: Namespace")
	}
	if saIdx < 0 {
		t.Fatal("RBAC output missing kind: ServiceAccount")
	}
	if nsIdx > saIdx {
		t.Error("Namespace must appear before ServiceAccount in RBAC output")
	}
}

func TestRBAC_ContainsExpectedResources(t *testing.T) {
	// Pass at least one rule so that ClusterRole + ClusterRoleBinding are emitted.
	someRule := rbacv1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"pods"},
		Verbs:     []string{"get"},
	}
	output, err := generate.RBAC([]rbacv1.PolicyRule{someRule}, nil, testNamespace, "", generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RBAC: %v", err)
	}
	s := string(output)

	for _, want := range []string{
		"kind: Namespace",
		"kind: ServiceAccount",
		"kind: ClusterRole",
		"kind: ClusterRoleBinding",
		testNamespace,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("RBAC output missing %q", want)
		}
	}
}

func TestRBAC_NamespaceAppearsOnce(t *testing.T) {
	output, err := generate.RBAC([]rbacv1.PolicyRule{}, nil, testNamespace, "", generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RBAC: %v", err)
	}

	if n := countOccurrences(string(output), "kind: Namespace"); n != 1 {
		t.Errorf("expected Namespace exactly once in RBAC output, got %d", n)
	}
}

// ── ConfigMap standalone ─────────────────────────────────────────────────────

func TestConfigMap_ContainsNamespaceFirst(t *testing.T) {
	output, err := generate.ConfigMap(minimalExpandedBlueprint(), testNamespace)
	if err != nil {
		t.Fatalf("ConfigMap: %v", err)
	}
	s := string(output)

	nsIdx := strings.Index(s, "kind: Namespace")
	cmIdx := strings.Index(s, "kind: ConfigMap")
	if nsIdx < 0 {
		t.Fatal("ConfigMap output missing kind: Namespace")
	}
	if cmIdx < 0 {
		t.Fatal("ConfigMap output missing kind: ConfigMap")
	}
	if nsIdx > cmIdx {
		t.Error("Namespace must appear before ConfigMap in output")
	}
}

func TestConfigMap_NamespaceAppearsOnce(t *testing.T) {
	output, err := generate.ConfigMap(minimalExpandedBlueprint(), testNamespace)
	if err != nil {
		t.Fatalf("ConfigMap: %v", err)
	}

	if n := countOccurrences(string(output), "kind: Namespace"); n != 1 {
		t.Errorf("expected Namespace exactly once in ConfigMap output, got %d", n)
	}
}

// ── Bundle ───────────────────────────────────────────────────────────────────

func TestRenderBundle_NamespaceAppearsOnce(t *testing.T) {
	bundle, err := generate.RenderBundle(nil, nil, minimalExpandedBlueprint(), testNamespace, "", generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RenderBundle: %v", err)
	}

	if n := countOccurrences(bundle, "kind: Namespace"); n != 1 {
		t.Errorf("expected Namespace exactly once in bundle, got %d\n\nBundle:\n%s", n, bundle)
	}
}

func TestRenderBundle_ContainsAllResources(t *testing.T) {
	bundle, err := generate.RenderBundle([]rbacv1.PolicyRule{}, nil, minimalExpandedBlueprint(), testNamespace, "", generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RenderBundle: %v", err)
	}

	for _, want := range []string{
		"kind: Namespace",
		"kind: ServiceAccount",
		"kind: ConfigMap",
		testNamespace,
	} {
		if !strings.Contains(bundle, want) {
			t.Errorf("bundle missing %q", want)
		}
	}
}

func TestRenderBundle_NamespaceBeforeEverythingElse(t *testing.T) {
	bundle, err := generate.RenderBundle(nil, nil, minimalExpandedBlueprint(), testNamespace, "", generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RenderBundle: %v", err)
	}

	nsIdx := strings.Index(bundle, "kind: Namespace")
	for _, kind := range []string{"kind: ServiceAccount", "kind: ClusterRole", "kind: ClusterRoleBinding", "kind: ConfigMap"} {
		idx := strings.Index(bundle, kind)
		if idx >= 0 && nsIdx > idx {
			t.Errorf("Namespace must appear before %s in bundle", kind)
		}
	}
}

func TestRenderBundle_WorkloadNamespace(t *testing.T) {
	workloadNS := "myapp-inrun-ns"

	bundle, err := generate.RenderBundle(nil, nil, minimalExpandedBlueprint(), testNamespace, workloadNS, generate.DefaultBundleOptions())
	if err != nil {
		t.Fatalf("RenderBundle: %v", err)
	}

	if n := countOccurrences(bundle, "kind: Namespace"); n != 2 {
		t.Errorf("expected 2 Namespace docs (operator + workload), got %d\n\nBundle:\n%s", n, bundle)
	}
	if !strings.Contains(bundle, workloadNS) {
		t.Errorf("bundle missing workload namespace %q", workloadNS)
	}
	if strings.Contains(bundle, "---\n---") {
		t.Errorf("bundle contains double --- separator\n\nBundle:\n%s", bundle)
	}
}

func TestRenderBundle_NoDoubleDocSeparators(t *testing.T) {
	for _, workloadNS := range []string{"", "myapp-inrun-ns"} {
		bundle, err := generate.RenderBundle(nil, nil, minimalExpandedBlueprint(), testNamespace, workloadNS, generate.DefaultBundleOptions())
		if err != nil {
			t.Fatalf("RenderBundle (workloadNS=%q): %v", workloadNS, err)
		}
		if strings.Contains(bundle, "---\n---") {
			t.Errorf("double --- separator found (workloadNS=%q)\n\nBundle:\n%s", workloadNS, bundle)
		}
	}
}
