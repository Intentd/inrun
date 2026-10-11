package prepare

import (
	"context"
	"testing"

	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// dynamicOnly is a kubeclient.Interface whose only working method is
// DynamicClient, which is all runMutation uses.
type dynamicOnly struct {
	kubeclient.Interface
	dyn dynamic.Interface
}

func (d dynamicOnly) DynamicClient() dynamic.Interface { return d.dyn }

// The mutation rules generated for serve labels land on the labels, including
// keys with dots and slashes: a default fills an absent label, an override
// replaces a present one.
func TestRunMutation_ServeLabelDefaultAndOverride(t *testing.T) {
	cr := newWebsite("default", "site", "example.com")
	cr.SetLabels(map[string]string{"tier": "silver"})

	dyn := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme(), cr)
	crd := types.CRDEntry{Serve: &types.ServeConfig{Labels: map[string]types.ServeFieldConfig{
		"app.kubernetes.io/name": {Default: "web"},
		"tier":                   {Override: "gold"},
	}}}
	rules := crd.ServeFieldMutationRules()

	res, err := runMutation(context.Background(), dynamicOnly{dyn: dyn}, cr,
		template.NewResolverFromMap(cr.Object), &types.MutationConfig{Rules: rules}, websiteGVR(), "Website")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 2 {
		t.Fatalf("applied %d changes, want 2: %+v", res.Applied, res.Changes)
	}

	got, err := dyn.Resource(websiteGVR()).Namespace("default").Get(context.Background(), "site", metav1.GetOptions{})
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
