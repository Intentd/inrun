package catalog

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUseUnstructured(t *testing.T) {
	typed := types.CRDEntry{
		Mode:     types.CRDModeTyped,
		APITypes: types.APITypes{Group: "demo.io", Version: "v1", Kind: "App", Location: "example.com/app/api/v1"},
	}
	builtIn := types.CRDEntry{
		IsBuiltIn: true,
		APITypes:  types.APITypes{Group: "apps", Version: "v1", Kind: "Deployment"},
	}
	cat := &Catalog{
		enabledCRDs: map[string]types.CRDEntry{"app": typed, "deploy": builtIn},
		Spec:        types.CatalogSpec{CRDs: map[string]types.CRDEntry{"app": typed}},
	}

	cat.UseUnstructured()

	for _, crds := range []map[string]types.CRDEntry{cat.enabledCRDs, cat.Spec.CRDs} {
		app := crds["app"]
		if !app.IsDynamic() || app.APITypes.Location != "" {
			t.Errorf("app: mode %q, location %q; want dynamic with no location", app.Mode, app.APITypes.Location)
		}
	}
	if d := cat.enabledCRDs["deploy"]; d.Mode != "" {
		t.Errorf("built-in type changed: mode %q", d.Mode)
	}

	// The typed CRD is now registered as unstructured, with no compiled type.
	scheme, err := cat.registerDynamicScheme(runtime.NewScheme())
	if err != nil {
		t.Fatal(err)
	}
	obj, err := scheme.New(schema.GroupVersionKind{Group: "demo.io", Version: "v1", Kind: "App"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.(*unstructured.Unstructured); !ok {
		t.Errorf("scheme.New = %T, want *unstructured.Unstructured", obj)
	}
}
