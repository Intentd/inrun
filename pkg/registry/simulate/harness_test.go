package simulate

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

const uniqueTestCRD = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: websites.testground.inrun.dev
spec:
  group: testground.inrun.dev
  names:
    kind: Website
    plural: websites
    singular: website
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [domain]
              properties:
                domain:
                  type: string
`

const uniqueTestBlueprint = `
apiVersion: inrun.dev/v1
kind: Blueprint
metadata:
  name: unique-operator-harness-test
  author: claude
  version: 0.1.0
  description: "harness test for operator: unique via ExistingInstances"

spec:
  crds:
    website:
      apiTypes:
        group: testground.inrun.dev
        version: v1alpha1
        kind: Website
        plural: websites
      crdFile: ./crd.yaml

      admission:
        validation:
          rules:
            - field: spec.domain
              unique: true
              message: "spec.domain must be unique across all Website instances"
              action: deny
`

// writeUniqueTestBlueprint materializes the blueprint+CRD fixture used by both
// TestRun_OperatorUnique_* tests into a temp dir, matching what
// merger.New/blueprint.BuildExpanded expect on disk.
func writeUniqueTestBlueprint(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "crd.yaml"), []byte(uniqueTestCRD), 0o644); err != nil {
		t.Fatalf("writing crd.yaml: %v", err)
	}
	blueprintPath := filepath.Join(dir, "blueprint.yaml")
	if err := os.WriteFile(blueprintPath, []byte(uniqueTestBlueprint), 0o644); err != nil {
		t.Fatalf("writing blueprint.yaml: %v", err)
	}
	return blueprintPath
}

func loadUniqueTestBlueprint(t *testing.T) *blueprint.Blueprint {
	t.Helper()
	blueprintPath := writeUniqueTestBlueprint(t)
	m := merger.New(blueprintPath)
	if err := m.Merge(); err != nil {
		t.Fatalf("merging blueprint: %v", err)
	}
	bp, err := pipeline.BuildExpanded(config.NewDefaultConfig(), m)
	if err != nil {
		t.Fatalf("building blueprint: %v", err)
	}
	return bp
}

func websiteCR(name, domain string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "testground.inrun.dev/v1alpha1",
		"kind":       "Website",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"domain": domain,
		},
	}}
}

// TestRun_OperatorUnique_NoDuplicate proves operator: unique passes end to
// end through the real reconcile pipeline (resolver → injected
// UniquenessChecker → validation.rules) when no other instance shares the
// field value.
func TestRun_OperatorUnique_NoDuplicate(t *testing.T) {
	bp := loadUniqueTestBlueprint(t)
	cr := websiteCR("site-a", "a.example.com")

	result, err := Run(context.Background(), bp, "website", cr, 1, RunOptions{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.Cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(result.Cycles))
	}
	if result.Cycles[0].Error != nil {
		t.Errorf("expected no reconcile error, got: %v", result.Cycles[0].Error)
	}
}

// TestRun_OperatorUnique_Duplicate proves the actual point of this feature:
// a pre-existing same-kind instance (RunOptions.ExistingInstances, from a
// second document of the CRD's own kind in a multi-doc CR file) is seeded
// into the fake dynamic client and makes operator: unique correctly deny —
// not a checker that trivially always passes because nothing was seeded.
func TestRun_OperatorUnique_Duplicate(t *testing.T) {
	bp := loadUniqueTestBlueprint(t)
	cr := websiteCR("site-b", "shared.example.com")
	existing := websiteCR("site-a", "shared.example.com")

	result, err := Run(context.Background(), bp, "website", cr, 1, RunOptions{
		ExistingInstances: []*unstructured.Unstructured{existing},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.Cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(result.Cycles))
	}
	if result.Cycles[0].Error == nil {
		t.Fatal("expected a validation-denied reconcile error, got none")
	}
}

func TestListKindsForSeeds_IncludesSeededCRD(t *testing.T) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "demo.inrun.dev", Version: "v1alpha1", Kind: "Website",
	})

	got := listKindsForSeeds(clientgoscheme.Scheme, []runtime.Object{u})

	gvr := schema.GroupVersionResource{
		Group: "demo.inrun.dev", Version: "v1alpha1", Resource: "websites",
	}
	assert.Equal(t, "WebsiteList", got[gvr])

	// And the built-in baseline is still present.
	cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	assert.Equal(t, "ConfigMapList", got[cmGVR])
}

func TestParseAPIRequest(t *testing.T) {
	cases := []struct {
		name          string
		method        string
		path          string
		wantVerb      string
		wantResource  string
		wantNamespace string
		wantName      string
	}{
		// Core group, namespaced.
		{"core ns named", "PATCH", "/api/v1/namespaces/default/configmaps/foo", "patch", "configmaps", "default", "foo"},
		{"core ns collection", "POST", "/api/v1/namespaces/default/configmaps", "", "", "", ""}, // handled by parseCollectionURL
		{"core ns subresource", "PUT", "/api/v1/namespaces/default/configmaps/foo/status", "update", "configmaps", "default", "foo"},

		// Core group, cluster-scoped.
		{"core cluster named", "PUT", "/api/v1/namespaces/default", "update", "namespaces", "", "default"},
		{"core cluster collection", "POST", "/api/v1/namespaces", "", "", "", ""}, // handled by parseCollectionURL

		// Non-core, namespaced.
		{"apps ns named", "PATCH", "/apis/apps/v1/namespaces/default/deployments/foo", "patch", "deployments", "default", "foo"},
		{"apps ns collection", "POST", "/apis/apps/v1/namespaces/default/deployments", "", "", "", ""}, // handled by parseCollectionURL
		{"apps ns subresource", "PUT", "/apis/apps/v1/namespaces/default/deployments/foo/status", "update", "deployments", "default", "foo"},

		// Non-core, cluster-scoped.
		{"rbac cluster named", "PUT", "/apis/rbac.authorization.k8s.io/v1/clusterroles/admin", "update", "clusterroles", "", "admin"},
		{"rbac cluster collection", "POST", "/apis/rbac.authorization.k8s.io/v1/clusterroles", "", "", "", ""}, // handled by parseCollectionURL

		// SSE apply.
		{"apply patch", "PATCH", "/apis/apps/v1/namespaces/default/deployments/foo", "apply", "deployments", "default", "foo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, "http://localhost"+tc.path, nil)
			if tc.wantVerb == "apply" {
				req.Header.Set("Content-Type", "application/apply-patch+yaml")
			}
			gotVerb, gotResource, gotNS, gotName := parseAPIRequest(req)
			assert.Equal(t, tc.wantVerb, gotVerb)
			assert.Equal(t, tc.wantResource, gotResource)
			assert.Equal(t, tc.wantNamespace, gotNS)
			assert.Equal(t, tc.wantName, gotName)
		})
	}
}

func TestParseCollectionURL(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		wantResource  string
		wantNamespace string
	}{
		{"core ns", "/api/v1/namespaces/default/configmaps", "configmaps", "default"},
		{"core cluster", "/api/v1/namespaces", "namespaces", ""},
		{"apps ns", "/apis/apps/v1/namespaces/default/deployments", "deployments", "default"},
		{"rbac cluster", "/apis/rbac.authorization.k8s.io/v1/clusterroles", "clusterroles", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "http://localhost"+tc.path, nil)
			gotResource, gotNS := parseCollectionURL(req)
			assert.Equal(t, tc.wantResource, gotResource)
			assert.Equal(t, tc.wantNamespace, gotNS)
		})
	}
}

func TestExtractMetaName_JSON(t *testing.T) {
	assert.Equal(t, "logs", extractMetaName([]byte(`{"metadata":{"name":"logs"}}`)))
}

// Typed clients get protobuf back for built-in kinds;
// a create's name must still be recorded.
func TestExtractMetaName_Protobuf(t *testing.T) {
	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-site-sa", Namespace: "default"},
	}
	info, ok := runtime.SerializerInfoForMediaType(clientgoscheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeProtobuf)
	require.True(t, ok)
	encoder := clientgoscheme.Codecs.EncoderForVersion(info.Serializer, corev1.SchemeGroupVersion)
	body, err := runtime.Encode(encoder, sa)
	require.NoError(t, err)

	assert.Equal(t, "my-site-sa", extractMetaName(body))
}

func TestExtractMetaName_Garbage(t *testing.T) {
	assert.Equal(t, "", extractMetaName([]byte("not an object")))
}

func TestPatchStatus_PersistsOnTheObject(t *testing.T) {
	f := NewFakeKubeclient(runtime.NewScheme())
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "storage.demo.inrun.dev/v1alpha1", "kind": "Volume",
		"metadata": map[string]interface{}{"name": "data", "namespace": "default"},
	}}
	require.NoError(t, f.PatchStatus(context.Background(), obj, map[string]interface{}{"volumeId": "v-1", "sizeGB": 10}, metav1.PatchOptions{}))
	got, _, _ := unstructured.NestedMap(obj.Object, "status")
	assert.Equal(t, map[string]interface{}{"volumeId": "v-1", "sizeGB": float64(10)}, got,
		"stored as the API server stores JSON")

	require.NoError(t, f.PatchStatus(context.Background(), obj, map[string]interface{}{"sizeGB": nil, "state": "ready"}, metav1.PatchOptions{}))
	got, _, _ = unstructured.NestedMap(obj.Object, "status")
	assert.Equal(t, map[string]interface{}{"volumeId": "v-1", "state": "ready"}, got,
		"a merge patch merges keys, and null removes one")

	require.NoError(t, f.PatchStatus(context.Background(), obj, nil, metav1.PatchOptions{}))
	assert.Len(t, f.Ops(), 2, "an empty patch is not sent, as with the real client")
}

func TestNoWrites(t *testing.T) {
	assert.True(t, noWrites(nil), "a pass that does nothing is steady")
	assert.True(t, noWrites([]Op{{Verb: "get"}, {Verb: "list"}}), "reads change nothing")
	assert.False(t, noWrites([]Op{{Verb: "get"}, {Verb: "patch", Resource: "status"}}))
	assert.False(t, noWrites([]Op{{Verb: "apply"}}))
}
