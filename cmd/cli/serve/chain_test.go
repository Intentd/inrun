//go:build !runtime && !gateway

package serve

import (
	"testing"

	"github.com/inrundev/inrun/pkg/catalog"
	"github.com/inrundev/inrun/pkg/types"
)

// chainTestCatalog builds a single-CRD catalog for kind "ServiceRequest" —
// its ServeTarget() defaults to the lowercased kind, "servicerequest",
// which every test below uses directly.
func chainTestCatalog(serveName string, tokens map[string]types.ServeTokenPermissions) *catalog.Catalog {
	k := catalog.NewFromEntryPointers(map[string]*types.CRDEntry{
		"servicerequest": {
			APITypes: types.APITypes{
				Group:   "demo.inrun.dev",
				Version: "v1",
				Kind:    "ServiceRequest",
				Plural:  "servicerequests",
			},
			Serve: &types.ServeConfig{
				Enabled:   true,
				Name:      serveName,
				Namespace: "default",
				Tokens:    tokens,
			},
		},
	})
	// Every token a test lists in serve.tokens is also a gateway token, as in a
	// real Catalog; "dev" is always declared.
	declared := []types.APIToken{{Name: "dev", Token: "dev-token"}}
	for name := range tokens {
		if name != "dev" {
			declared = append(declared, types.APIToken{Name: name, Token: name + "-token"})
		}
	}
	k.Gateway = &types.GatewayConfig{Enabled: true, API: &types.GatewayAPIConfig{
		Enabled: true,
		Auth:    types.APIAuth{Tokens: declared},
	}}
	return k
}

func TestPlayChain_UndeclaredTokenDenied(t *testing.T) {
	// No serve.tokens on the target: any token declared in
	// gateway.api.auth.tokens may create, but an undeclared name may not.
	k := chainTestCatalog("", nil)
	_, _, _, err := planChain(
		k, map[string]interface{}{"target": "servicerequest", "name": "x"}, "unknown", types.ServeOpCreate, quietNarrator{},
	)
	if err == nil {
		t.Fatal("expected a token missing from gateway.api.auth.tokens to be denied")
	}
}

func TestPlayChain_UnknownTarget(t *testing.T) {
	k := chainTestCatalog("", nil)
	_, _, _, err := planChain(
		k, map[string]interface{}{"target": "does-not-exist"}, "dev", types.ServeOpCreate, quietNarrator{},
	)
	if err == nil {
		t.Fatal("expected an error for an unknown target")
	}
}

func TestPlayChain_MissingTarget(t *testing.T) {
	k := chainTestCatalog("", nil)
	_, _, _, err := planChain(
		k, map[string]interface{}{"name": "x"}, "dev", types.ServeOpCreate, quietNarrator{},
	)
	if err == nil {
		t.Fatal(`expected an error when "target" is absent`)
	}
}

func TestPlayChain_TokenDenied(t *testing.T) {
	k := chainTestCatalog("", map[string]types.ServeTokenPermissions{
		"dev": {Permissions: types.ServePermissionSet{Global: []string{"get"}}}, // no create
	})
	_, _, _, err := planChain(
		k, map[string]interface{}{"target": "servicerequest", "name": "x"}, "dev", types.ServeOpCreate, quietNarrator{},
	)
	if err == nil {
		t.Fatal("expected the token to be denied create")
	}
}

func TestPlayChain_UnknownTokenDenied(t *testing.T) {
	// Once ANY token is declared, an unlisted token name is denied outright —
	// no implicit fallback to "allow all".
	k := chainTestCatalog("", map[string]types.ServeTokenPermissions{
		"dev": {Permissions: types.ServePermissionSet{Global: []string{"*"}}},
	})
	_, _, _, err := planChain(
		k, map[string]interface{}{"target": "servicerequest", "name": "x"}, "someone-else", types.ServeOpCreate, quietNarrator{},
	)
	if err == nil {
		t.Fatal("expected an unlisted token name to be denied")
	}
}

func TestPlayChain_BuildsCRWithProvenance(t *testing.T) {
	k := chainTestCatalog("", nil) // no serve.tokens: any declared gateway token may create
	obj, crd, alias, err := planChain(
		k,
		map[string]interface{}{"target": "servicerequest", "name": "payments-api"},
		"dev", types.ServeOpCreate, quietNarrator{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.GetName() != "payments-api" || obj.GetNamespace() != "default" {
		t.Errorf("name/namespace = %s/%s", obj.GetName(), obj.GetNamespace())
	}
	if crd.Kind() != "ServiceRequest" {
		t.Errorf("Kind = %q", crd.Kind())
	}
	if alias != "" {
		t.Errorf("alias = %q, want empty (primary target)", alias)
	}
	if got := obj.GetAnnotations()["inrun.dev/serve-target"]; got != "servicerequest" {
		t.Errorf("serve-target annotation = %q, want servicerequest", got)
	}
}

func TestPlayChain_MissingNameRejected(t *testing.T) {
	// serve.name isn't declared and no raw "name" is supplied.
	k := chainTestCatalog("", nil)
	_, _, _, err := planChain(
		k, map[string]interface{}{"target": "servicerequest"}, "dev", types.ServeOpCreate, quietNarrator{},
	)
	if err == nil {
		t.Fatal("expected an error when neither serve.name nor a raw name is available")
	}
}
