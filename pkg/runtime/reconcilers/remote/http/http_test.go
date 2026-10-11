package http

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newReconcilerForTest(endpoint string) *Reconciler {
	return &Reconciler{
		decl: &types.RemoteReconcilerDeclaration{
			Endpoint: endpoint,
		},
	}
}

func reqWithResolver(data map[string]interface{}) domain.Request {
	resolver := template.NewResolverFromMap(data)
	return domain.Request{
		Prepared: &domain.PreparedRequest{Context: resolver},
	}
}

func TestResolveEndpoint_Static(t *testing.T) {
	r := newReconcilerForTest("http://svc.internal/reconcile")
	assert.Equal(t, "http://svc.internal/reconcile", r.resolveEndpoint(domain.Request{}))
}

func TestResolveEndpoint_Template(t *testing.T) {
	r := newReconcilerForTest("http://{{ index . \"namespace\" }}-svc/reconcile")
	req := reqWithResolver(map[string]interface{}{"namespace": "prod"})
	assert.Equal(t, "http://prod-svc/reconcile", r.resolveEndpoint(req))
}

func TestResolveEndpoint_NilPrepared_FallsBack(t *testing.T) {
	r := newReconcilerForTest("http://{{ .metadata.namespace }}-svc/reconcile")
	assert.Equal(t, "http://{{ .metadata.namespace }}-svc/reconcile", r.resolveEndpoint(domain.Request{}))
}

func TestApplyObjectExclusions_RemovesPaths(t *testing.T) {
	r := &Reconciler{
		decl: &types.RemoteReconcilerDeclaration{
			Endpoint: "http://svc/reconcile",
			Request: &types.RemoteRequestConfig{
				Object: &types.RemoteRequestObjectConfig{
					Exclude: []string{"metadata.managedFields", "metadata.annotations"},
				},
			},
		},
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":          "my-app",
			"managedFields": []interface{}{"some-data"},
			"annotations":   map[string]interface{}{"kubectl.io/last-applied": "{}"},
		},
		"spec": map[string]interface{}{"replicas": float64(2)},
	}}
	got := r.applyObjectExclusions(obj, r.decl.Request.Object.Exclude)
	meta := got["metadata"].(map[string]interface{})
	assert.Equal(t, "my-app", meta["name"])
	assert.Nil(t, meta["managedFields"])
	assert.Nil(t, meta["annotations"])
	assert.NotNil(t, got["spec"])
}

func TestApplyObjectExclusions_NoExclusions_ReturnsOriginalMap(t *testing.T) {
	r := newReconcilerForTest("http://svc/reconcile")
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "my-app"},
	}}
	got := r.applyObjectExclusions(obj, nil)
	assert.Equal(t, "my-app", got["metadata"].(map[string]interface{})["name"])
}

func TestApplyObjectExclusions_DoesNotMutateOriginal(t *testing.T) {
	r := &Reconciler{
		decl: &types.RemoteReconcilerDeclaration{
			Endpoint: "http://svc/reconcile",
		},
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":          "my-app",
			"managedFields": []interface{}{"data"},
		},
		"spec": map[string]interface{}{"image": "nginx:1.25"},
	}}
	_ = r.applyObjectExclusions(obj, []string{"metadata.managedFields"})
	meta := obj.Object["metadata"].(map[string]interface{})
	assert.NotNil(t, meta["managedFields"], "original object must not be mutated")
}

func TestApplyChildrenConfig_ResourcesFilter(t *testing.T) {
	r := &Reconciler{
		decl: &types.RemoteReconcilerDeclaration{
			Request: &types.RemoteRequestConfig{
				Children: &types.RemoteRequestChildrenConfig{
					Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
						"deployment": {},
					},
				},
			},
		},
	}
	cm := childrenMap{
		"deployment": {"app": {"kind": "Deployment"}},
		"service":    {"svc": {"kind": "Service"}},
	}
	got := r.applyChildrenConfig(cm)
	assert.NotNil(t, got["deployment"])
	assert.Nil(t, got["service"])
}

func TestApplyChildrenConfig_RootExcludeStripsPath(t *testing.T) {
	r := &Reconciler{
		decl: &types.RemoteReconcilerDeclaration{
			Request: &types.RemoteRequestConfig{
				Children: &types.RemoteRequestChildrenConfig{
					Exclude: []string{"metadata.managedFields"},
				},
			},
		},
	}
	cm := childrenMap{
		"deployment": {
			"app": {
				"kind": "Deployment",
				"metadata": map[string]interface{}{
					"name":          "app",
					"managedFields": []interface{}{"data"},
				},
			},
		},
	}
	got := r.applyChildrenConfig(cm)
	meta := got["deployment"]["app"]["metadata"].(map[string]interface{})
	assert.Nil(t, meta["managedFields"])
	assert.Equal(t, "app", meta["name"])
}

func TestApplyChildrenConfig_PerResourceExcludeOverridesRoot(t *testing.T) {
	r := &Reconciler{
		decl: &types.RemoteReconcilerDeclaration{
			Request: &types.RemoteRequestConfig{
				Children: &types.RemoteRequestChildrenConfig{
					Exclude: []string{"metadata.managedFields"},
					Resources: map[string]*types.RemoteRequestChildrenResourceConfig{
						"deployment": {Exclude: []string{"spec.template.metadata.annotations"}},
						"service":    nil, // uses root exclude
					},
				},
			},
		},
	}
	cm := childrenMap{
		"deployment": {
			"app": {
				"metadata": map[string]interface{}{"managedFields": []interface{}{"x"}},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{"annotations": map[string]interface{}{"k": "v"}},
					},
				},
			},
		},
		"service": {
			"svc": {
				"metadata": map[string]interface{}{"managedFields": []interface{}{"x"}, "name": "svc"},
			},
		},
	}
	got := r.applyChildrenConfig(cm)
	// deployment: per-resource exclude strips spec.template.metadata.annotations, NOT managedFields
	depMeta := got["deployment"]["app"]["metadata"].(map[string]interface{})
	assert.NotNil(t, depMeta["managedFields"])
	depTplMeta := got["deployment"]["app"]["spec"].(map[string]interface{})["template"].(map[string]interface{})["metadata"].(map[string]interface{})
	assert.Nil(t, depTplMeta["annotations"])
	// service: root exclude strips managedFields
	svcMeta := got["service"]["svc"]["metadata"].(map[string]interface{})
	assert.Nil(t, svcMeta["managedFields"])
}

func TestBuildBody_ExplicitWireForm(t *testing.T) {
	disabled := false
	r := &Reconciler{
		gvk: schema.GroupVersionKind{Group: "demo.inrun.dev", Version: "v1alpha1", Kind: "Website"},
		decl: &types.RemoteReconcilerDeclaration{
			Endpoint: "http://svc/reconcile",
			Request: &types.RemoteRequestConfig{
				Object: &types.RemoteRequestObjectConfig{Exclude: []string{"spec"}},
				// Listing children needs a kube client; covered by the children tests.
				Children: &types.RemoteRequestChildrenConfig{Enabled: &disabled},
			},
		},
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "a", "namespace": "default"},
		"spec":     map[string]interface{}{"image": "nginx"},
	}}
	req := domain.Request{
		Key:      "default/a",
		Prepared: &domain.PreparedRequest{Object: obj, Target: "website", Args: domain.Args{"image": "nginx"}},
	}

	raw, err := r.buildBody(req)
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body))

	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"key", "apiVersion", "kind", "target", "object", "args"}, keys)
	assert.Equal(t, "demo.inrun.dev/v1alpha1", body["apiVersion"])
	assert.Equal(t, "Website", body["kind"])
	assert.Equal(t, "website", body["target"])
	assert.NotContains(t, body["object"], "spec", "excluded paths must not reach the reconciler")
	assert.NotContains(t, string(raw), `"spec"`, "no other copy of the CR in the body")

	// No prepared state: identity only, and no panic.
	raw, err = r.buildBody(domain.Request{Key: "default/a"})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"key":"default/a"`)
}
