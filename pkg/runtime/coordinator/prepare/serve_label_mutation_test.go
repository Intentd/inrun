package prepare

import (
	"context"
	"github.com/intentd/inrun/pkg/kubeclient/kubefake"
	"testing"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The mutation rules generated for serve labels land on the labels, including
// keys with dots and slashes: a default fills an absent label, an override
// replaces a present one.
func TestRunMutation_ServeLabelDefaultAndOverride(t *testing.T) {
	cr := newWebsite("default", "site", "example.com")
	cr.SetLabels(map[string]string{"tier": "silver"})

	kube := kubefake.New(runtime.NewScheme(), cr)
	crd := types.CRDEntry{Serve: &types.ServeConfig{Labels: map[string]types.ServeFieldConfig{
		"app.kubernetes.io/name": {Default: "web"},
		"tier":                   {Override: "gold"},
	}}}
	rules := crd.ServeFieldMutationRules()

	res, err := runMutation(context.Background(), kube, cr,
		template.NewResolverFromMap(cr.Object), &types.MutationConfig{Rules: rules}, websiteGVR(), "Website")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 2 {
		t.Fatalf("applied %d changes, want 2: %+v", res.Applied, res.Changes)
	}

	got, err := kube.DynamicClient().Resource(websiteGVR()).Namespace("default").Get(context.Background(), "site", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"app.kubernetes.io/name": "web", "tier": "gold"}
	for k, v := range want {
		if got.GetLabels()[k] != v {
			t.Errorf("labels = %v, want %s=%s", got.GetLabels(), k, v)
		}
	}
	if _, nested := got.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["app"]; nested {
		t.Error("the dotted key was split into nested maps")
	}
}
