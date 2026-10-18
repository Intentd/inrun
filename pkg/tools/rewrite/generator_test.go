package rewrite

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/pkg/catalog"
)

// The generated Catalog must decode strictly: an unknown or misplaced key
// (an old layout) fails here instead of at the user's first inrun validate.
func TestGenerateCatalog_DecodesStrictly(t *testing.T) {
	res := &Result{
		PkgName: "controllers",
		Owns:    []DetectedType{{Kind: "Deployment", APIVersion: "apps/v1"}, {Kind: "Service"}},
		Watches: []DetectedType{{Kind: "ConfigMap", APIVersion: "v1"}},
		Primary: PrimaryType{Kind: "WebApp", Object: "WebApp", ObjectList: "WebAppList", Version: "v1alpha1"},
	}
	opts := Options{ModulePath: "github.com/acme/webapp", OperatorName: "webapp"}

	out := generateCatalog(res, opts, "webapp", "NewWebAppReconciler")

	var cat catalog.Catalog
	if err := structured.StrictUnmarshal([]byte(out), &cat); err != nil {
		t.Fatalf("generated catalog does not decode:\n%v\n\n%s", err, out)
	}
	for _, want := range []string{"reconcile:", "managedResources:", "constructor:", "observe:", "allowedNamespaces:"} {
		if !strings.Contains(out, want) {
			t.Errorf("generated catalog is missing %q", want)
		}
	}
}
