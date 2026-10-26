package simulate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake/object"
	"github.com/intentd/inrun/pkg/types"

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
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
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
	return object.New("testground.inrun.dev/v1alpha1", "Website", name, object.InNamespace("default"),
		object.WithSpec(map[string]any{"domain": domain}))
}

// TestRun_OperatorUnique_NoDuplicate proves operator: unique passes end to
// end through the real reconcile pipeline (resolver → injected
// UniquenessChecker → validation.rules) when no other instance shares the
// field value.
func TestRun_OperatorUnique_NoDuplicate(t *testing.T) {
	bp := loadUniqueTestBlueprint(t)
	cr := websiteCR("site-a", "a.example.com")

	result, err := run(context.Background(), RunOptions{Blueprint: bp, CRDName: "website", CR: cr, MaxCycles: 1})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
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

	result, err := run(context.Background(), RunOptions{
		Blueprint: bp, CRDName: "website", CR: cr, MaxCycles: 1,
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
	u := object.New("demo.inrun.dev/v1alpha1", "Website", "")

	got := kubefake.ListKindsForSeeds(clientgoscheme.Scheme, []runtime.Object{u})

	gvr := schema.GroupVersionResource{
		Group: "demo.inrun.dev", Version: "v1alpha1", Resource: "websites",
	}
	assert.Equal(t, "WebsiteList", got[gvr])

	// And the built-in baseline is still present.
	cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	assert.Equal(t, "ConfigMapList", got[cmGVR])
}

// TestDescribeWrite reads write requests the way the API server routes
// them.
func TestDescribeWrite(t *testing.T) {
	cases := []struct {
		name, method, path string
		apply              bool
		want               apiWrite
		wantOK             bool
	}{
		// Core group.
		{"core ns named", "PATCH", "/api/v1/namespaces/default/configmaps/foo", false, apiWrite{"patch", "", "v1", "configmaps", "default", "foo"}, true},
		{"core ns subresource", "PUT", "/api/v1/namespaces/default/pods/foo/status", false, apiWrite{"update", "", "v1", "pods", "default", "foo"}, true},
		{"core ns collection create", "POST", "/api/v1/namespaces/default/configmaps", false, apiWrite{"create", "", "v1", "configmaps", "default", ""}, true},
		{"namespace object", "PUT", "/api/v1/namespaces/team", false, apiWrite{"update", "", "v1", "namespaces", "", "team"}, true},
		{"namespace status", "PUT", "/api/v1/namespaces/team/status", false, apiWrite{"update", "", "v1", "namespaces", "", "team"}, true},
		{"namespace finalize", "PUT", "/api/v1/namespaces/team/finalize", false, apiWrite{"update", "", "v1", "namespaces", "", "team"}, true},
		{"namespace create", "POST", "/api/v1/namespaces", false, apiWrite{"create", "", "v1", "namespaces", "", ""}, true},

		// Grouped, namespaced.
		{"apps ns named", "DELETE", "/apis/apps/v1/namespaces/default/deployments/foo", false, apiWrite{"delete", "apps", "v1", "deployments", "default", "foo"}, true},
		{"apps ns subresource", "PUT", "/apis/apps/v1/namespaces/default/deployments/foo/status", false, apiWrite{"update", "apps", "v1", "deployments", "default", "foo"}, true},
		{"apply", "PATCH", "/apis/apps/v1/namespaces/default/deployments/foo", true, apiWrite{"apply", "apps", "v1", "deployments", "default", "foo"}, true},

		// Grouped, cluster-scoped.
		{"cluster named", "PUT", "/apis/rbac.authorization.k8s.io/v1/clusterroles/admin", false, apiWrite{"update", "rbac.authorization.k8s.io", "v1", "clusterroles", "", "admin"}, true},
		{"cluster subresource", "PATCH", "/apis/demo.inrun.dev/v1/tenants/acme/status", false, apiWrite{"patch", "demo.inrun.dev", "v1", "tenants", "", "acme"}, true},
		{"cluster collection create", "POST", "/apis/demo.inrun.dev/v1/tenants", false, apiWrite{"create", "demo.inrun.dev", "v1", "tenants", "", ""}, true},

		// Not writes to one object.
		{"get", "GET", "/apis/apps/v1/namespaces/default/deployments/foo", false, apiWrite{}, false},
		{"delete collection", "DELETE", "/apis/apps/v1/namespaces/default/deployments", false, apiWrite{}, false},
		{"non-resource", "POST", "/healthz", false, apiWrite{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, "http://localhost"+tc.path, nil)
			if tc.apply {
				req.Header.Set("Content-Type", "application/apply-patch+yaml")
			}
			got, ok := describeWrite(req)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
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

// TestPatchStatus_PersistsInTheStore: a status patch is applied to the
// stored object as the API server applies a merge patch, numbers as int64,
// and leaves the caller's object as it was, as the real client does.
func TestPatchStatus_PersistsInTheStore(t *testing.T) {
	obj := object.New("storage.demo.inrun.dev/v1alpha1", "Volume", "data", object.InNamespace("default"))
	obj.SetGeneration(1)
	f := kubefake.New(runtime.NewScheme(), obj.DeepCopy())
	gvr := schema.GroupVersionResource{Group: "storage.demo.inrun.dev", Version: "v1alpha1", Resource: "volumes"}
	stored := func() *unstructured.Unstructured {
		u, err := f.Object(context.Background(), gvr, "default", "data")
		require.NoError(t, err)
		return u
	}

	require.NoError(t, f.PatchStatus(context.Background(), obj, map[string]interface{}{"volumeId": "v-1", "sizeGB": 10}, metav1.PatchOptions{}))
	got, _, _ := unstructured.NestedMap(stored().Object, "status")
	assert.Equal(t, map[string]interface{}{"volumeId": "v-1", "sizeGB": int64(10)}, got,
		"stored as the API server stores JSON")
	assert.Equal(t, int64(1), stored().GetGeneration(), "generation survives a status patch")
	assert.NotContains(t, obj.Object, "status", "the caller's object is left as it was")

	require.NoError(t, f.PatchStatus(context.Background(), obj, map[string]interface{}{"sizeGB": nil, "state": "ready"}, metav1.PatchOptions{}))
	got, _, _ = unstructured.NestedMap(stored().Object, "status")
	assert.Equal(t, map[string]interface{}{"volumeId": "v-1", "state": "ready"}, got,
		"a merge patch merges keys, and null removes one")

	require.NoError(t, f.PatchStatus(context.Background(), obj, nil, metav1.PatchOptions{}))
	assert.Len(t, f.Ops(), 2, "an empty patch is not sent, as with the real client")
}

func TestNoWrites(t *testing.T) {
	assert.True(t, noWrites(nil), "a pass that does nothing is steady")
	assert.True(t, noWrites(Ops{{Verb: "get"}, {Verb: "list"}}), "reads change nothing")
	assert.False(t, noWrites(Ops{{Verb: "get"}, {Verb: "patch", Resource: "status"}}))
	assert.False(t, noWrites(Ops{{Verb: "apply"}}))
}

func TestSetupObjects_CRDThenSpec(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		return p
	}
	crdSetup := write("token.yaml", "apiVersion: v1\nkind: Secret\nmetadata: {name: token}\n")
	specSetup := write("namespaces.yaml", "apiVersion: v1\nkind: Namespace\nmetadata: {name: team-a}\n---\napiVersion: v1\nkind: Namespace\nmetadata: {name: team-b}\n")

	crd := &types.CRDEntry{Setup: &types.SetupConfig{Apply: []types.SetupApplyEntry{{Path: crdSetup}}}}
	objs, err := setupObjects(crd, []string{specSetup})
	require.NoError(t, err)
	var names []string
	for _, o := range objs {
		names = append(names, o.GetKind()+"/"+o.GetName())
	}
	assert.Equal(t, []string{"Secret/token", "Namespace/team-a", "Namespace/team-b"}, names)

	objs, err = setupObjects(&types.CRDEntry{}, nil)
	require.NoError(t, err)
	assert.Empty(t, objs)
}

// TestRecordingTransport_NamesLikeTheFakeClient sends writes through the
// real kubeclient to a stub API server and checks the recorder names each
// the way kubefake does: by the kubeclient method, never by the patch body.
func TestRecordingTransport_NamesLikeTheFakeClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api":
			fmt.Fprint(w, `{"kind":"APIVersions","versions":["v1"]}`)
		case "/apis":
			fmt.Fprint(w, `{"kind":"APIGroupList","groups":[]}`)
		case "/api/v1":
			fmt.Fprint(w, `{"kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"configmaps","namespaced":true,"kind":"ConfigMap","verbs":["get","patch"]}]}`)
		default:
			fmt.Fprint(w, `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"web","namespace":"team"}}`)
		}
	}))
	defer srv.Close()

	shared := &sharedOps{}
	cfg := &rest.Config{Host: srv.URL}
	cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return &recordingTransport{inner: rt, shared: shared}
	}
	dyn, err := dynamic.NewForConfig(cfg)
	require.NoError(t, err)
	kube := kubeclient.NewForTesting(cfg, dyn, clientgoscheme.Scheme)

	ctx := context.Background()
	cm := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "team"},
	}
	opts := metav1.PatchOptions{}
	require.NoError(t, kube.PatchFinalizers(ctx, cm, []string{"inrun.dev/cleanup"}, opts))
	require.NoError(t, kube.PatchLabels(ctx, cm, nil, map[string]string{"a": "b"}, opts))
	require.NoError(t, kube.PatchAnnotations(ctx, cm, map[string]string{"c": "d"}, opts))
	// A labels-only patch from elsewhere, as a mutation rule sends it: the fake
	// client's reactor records it under its resource, so must the recorder.
	_, err = dyn.Resource(corev1.SchemeGroupVersion.WithResource("configmaps")).Namespace("team").
		Patch(ctx, "web", k8stypes.MergePatchType, []byte(`{"metadata":{"labels":{"tier":"web"}}}`), opts)
	require.NoError(t, err)

	var got []string
	for _, op := range shared.ops {
		got = append(got, op.Verb+" "+op.Resource+"/"+op.Name)
	}
	assert.Equal(t, []string{
		"patch finalizers/web",
		"patch labels/web",
		"patch annotations/web",
		"patch configmaps/web",
	}, got)
}

func TestPatchedPart_UntaggedWriteKeepsResource(t *testing.T) {
	r, _ := http.NewRequest(http.MethodPatch, "https://api/apis/demo.inrun.dev/v1/namespaces/team/apps/web/status", nil)
	assert.Equal(t, "apps", patchedPart(r, "apps"), "a status path alone is not PatchStatus")
}

// TestRun_TargetReachesTheReconciledCR proves RunOptions.Target is applied
// to the CR the run seeds and reconciles, not to a copy thrown away, and
// that the caller's CR is left as it was.
func TestRun_TargetReachesTheReconciledCR(t *testing.T) {
	bp := loadUniqueTestBlueprint(t)
	cr := websiteCR("site-a", "a.example.com")

	result, err := New(context.Background(), RunOptions{Blueprint: bp, CRDName: "website", CR: cr, MaxCycles: 1, Target: "edge"}).Execute(InMemory)
	require.NoError(t, err)
	require.NotEmpty(t, result.Final, "the CR is read back after the run")

	assert.Equal(t, "edge", intent.Target(result.Final[0].GetAnnotations()), "the reconciled CR carries the target")
	assert.Empty(t, intent.Target(cr.GetAnnotations()), "the caller's CR is untouched")
}

// TestRun_NotesAMissingCRDFile: without the CRD's file the in-memory run
// still works, and its result says schema defaults and scope were not
// applied; with the file there is no note.
func TestRun_NotesAMissingCRDFile(t *testing.T) {
	bp := loadUniqueTestBlueprint(t)
	hasNote := func(r *Result) bool {
		for _, n := range r.Notes {
			if strings.Contains(n, "no CRD file for Website") {
				return true
			}
		}
		return false
	}

	without, err := New(context.Background(), RunOptions{Blueprint: bp, CRDName: "website", CR: websiteCR("site-a", "a.example.com"), MaxCycles: 1}).Execute(InMemory)
	require.NoError(t, err)
	assert.True(t, hasNote(without), "notes: %v", without.Notes)

	crdPath := filepath.Join(filepath.Dir(writeUniqueTestBlueprint(t)), "crd.yaml")
	with, err := New(context.Background(), RunOptions{Blueprint: bp, CRDName: "website", CR: websiteCR("site-a", "a.example.com"), MaxCycles: 1, CRDPaths: []string{crdPath}}).Execute(InMemory)
	require.NoError(t, err)
	assert.False(t, hasNote(with), "notes: %v", with.Notes)
}
