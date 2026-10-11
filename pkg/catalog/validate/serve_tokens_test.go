package validate

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/catalog"

	"github.com/intentd/inrun/pkg/types"
)

// Helper to create a Catalog with gateway tokens
func catalogWithGatewayTokens(tokens ...string) *executor {
	applyTokens := make([]types.APIToken, len(tokens))
	for i, name := range tokens {
		applyTokens[i] = types.APIToken{Name: name}
	}

	return newExec(&catalog.Catalog{
		Gateway: &types.GatewayConfig{
			Enabled: true,
			API: &types.GatewayAPIConfig{
				Enabled: true,
				Auth: types.APIAuth{
					Tokens: applyTokens,
				},
			},
		},
	})
}

// Helper to create token permissions with global list
func perms(ops ...string) types.ServeTokenPermissions {
	return types.ServeTokenPermissions{
		Permissions: types.ServePermissionSet{
			Global: ops,
		},
	}
}

// Helper to create token permissions with schema and resources lists
func permsWithScopes(schemaOps, resourceOps []string) types.ServeTokenPermissions {
	return types.ServeTokenPermissions{
		Permissions: types.ServePermissionSet{
			Schema:    schemaOps,
			Resources: resourceOps,
		},
	}
}

// Helper to create token permissions with all three lists
func permsWithAll(globalOps, schemaOps, resourceOps []string) types.ServeTokenPermissions {
	return types.ServeTokenPermissions{
		Permissions: types.ServePermissionSet{
			Global:    globalOps,
			Schema:    schemaOps,
			Resources: resourceOps,
		},
	}
}

// Helper to create token permissions with namespaces
func permsWithNamespaces(ops []string, namespaces ...string) types.ServeTokenPermissions {
	return types.ServeTokenPermissions{
		Permissions: types.ServePermissionSet{
			Global: ops,
		},
		Namespaces: namespaces,
	}
}

func TestValidateServeTokenRestrictions_NilServe(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{
		"myresource": {Serve: nil},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_NoTokenRestrictions(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{
		"myresource": {Serve: &types.ServeConfig{Enabled: true}},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_TokenExists_Global(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline", "console")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": perms("get", "list"),
				},
			},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_TokenExists_Scoped(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline", "console")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithScopes(
						[]string{"get"},              // schema
						[]string{"create", "update"}, // resources
					),
				},
			},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_TokenExists_GlobalWithScopes(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline", "console")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithAll(
						[]string{"get", "list", "create", "update"}, // global
						[]string{"get"},              // schema (subset)
						[]string{"create", "update"}, // resources (subset)
					),
				},
			},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_TokenNotFound(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"unknown-token": perms("get"),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
	if !strings.Contains(err.Error(), "unknown-token") {
		t.Errorf("error should mention the unknown token, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ci-pipeline") {
		t.Errorf("error should list available tokens, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_InvalidOperation_Global(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": perms("get", "invalid-op"),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for invalid operation")
	}
	if !strings.Contains(err.Error(), "invalid-op") {
		t.Errorf("error should mention the invalid operation, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_InvalidOperation_Schema(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithScopes(
						[]string{"invalid-op"}, // schema
						[]string{"create"},     // resources
					),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for invalid schema operation")
	}
	if !strings.Contains(err.Error(), "invalid-op") {
		t.Errorf("error should mention the invalid operation, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_InvalidOperation_Resources(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithScopes(
						[]string{"get"},        // schema
						[]string{"invalid-op"}, // resources
					),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for invalid resources operation")
	}
	if !strings.Contains(err.Error(), "invalid-op") {
		t.Errorf("error should mention the invalid operation, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_DuplicateOperations(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": perms("get", "list", "get"),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for duplicate operations")
	}
	if !strings.Contains(err.Error(), "get") {
		t.Errorf("error should mention the duplicate operation, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_SchemaNotSubsetOfGlobal(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithAll(
						[]string{"get", "list"}, // global
						[]string{"delete"},      // schema (not in global)
						[]string{"get"},         // resources
					),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error when schema is not subset of global")
	}
	if !strings.Contains(err.Error(), "delete") {
		t.Errorf("error should mention the missing operation, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_ResourcesNotSubsetOfGlobal(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithAll(
						[]string{"get", "list"}, // global
						[]string{"get"},         // schema
						[]string{"delete"},      // resources (not in global)
					),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error when resources is not subset of global")
	}
	if !strings.Contains(err.Error(), "delete") {
		t.Errorf("error should mention the missing operation, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_ClassWildcardWithoutGlobalWildcard(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithAll(
						[]string{"get", "list"}, // global
						[]string{"*"},           // schema wildcard (broader than global)
						[]string{"get"},         // resources
					),
				},
			},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error when class wildcard exceeds global")
	}
	if !strings.Contains(err.Error(), "*") {
		t.Errorf("error should mention wildcard, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_GlobalWildcard(t *testing.T) {
	e := catalogWithGatewayTokens("admin")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"admin": perms("*"),
				},
			},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_GlobalWildcardWithScopes(t *testing.T) {
	// When global has "*", class-specific scopes should be ignored.
	// The validation should not error because global overrides everything.
	e := catalogWithGatewayTokens("admin")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"admin": permsWithAll(
						[]string{"*"},      // global wildcard
						[]string{"get"},    // schema (ignored)
						[]string{"create"}, // resources (ignored)
					),
				},
			},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_NamespaceAllowed(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithNamespaces(
						[]string{"get"},
						"staging",
					),
				},
			},
			OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{AllowedNamespaces: []string{"staging", "production"}}},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_NamespaceNotAllowed(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithNamespaces(
						[]string{"get"},
						"unauthorized",
					),
				},
			},
			OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{AllowedNamespaces: []string{"staging", "production"}}},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for unauthorized namespace")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error should mention the unauthorized namespace, got: %v", err)
	}
	if !strings.Contains(err.Error(), "staging") {
		t.Errorf("error should list allowed namespaces, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_RestrictedNamespace(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithNamespaces(
						[]string{"get"},
						"restricted-ns",
					),
				},
			},
			OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{RestrictedNamespaces: []string{"restricted-ns", "blocked"}}},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error for restricted namespace")
	}
	if !strings.Contains(err.Error(), "restricted-ns") {
		t.Errorf("error should mention the restricted namespace, got: %v", err)
	}
}

func TestValidateServeTokenRestrictions_MultipleTokens(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline", "console")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": perms("get", "list"),
					"console":     perms("*"),
				},
			},
			OperatorBox: &types.OperatorBoxConfig{Runtime: &types.RuntimeConfig{AllowedNamespaces: []string{"staging", "production"}}},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_WildcardOperation(t *testing.T) {
	e := catalogWithGatewayTokens("admin")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"admin": perms("*"),
				},
			},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeTokenRestrictions_GatewayNotConfigured(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": perms("get"),
				},
			},
			Warnings: types.Warnings{},
		},
	})
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error when gateway is not configured but serve tokens are defined")
	}
}

func TestValidateServeTokenRestrictions_GatewayAuthEmpty(t *testing.T) {
	e := newCatalogExec(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": perms("get"),
				},
			},
			Warnings: types.Warnings{},
		},
	})
	e.cat.Gateway = &types.GatewayConfig{
		API: &types.GatewayAPIConfig{
			Auth: types.APIAuth{
				Tokens: []types.APIToken{},
			},
		},
	}
	err := e.validateServeTokenRestrictions()
	if err == nil {
		t.Fatal("expected error when gateway auth tokens are empty but serve tokens are defined")
	}
}

func TestGatewayTokenNames(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline", "console")
	names := e.cat.GatewayTokenNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(names), names)
	}
	if names[0] != "ci-pipeline" || names[1] != "console" {
		t.Errorf("unexpected token names: %v", names)
	}
}

func TestGatewayTokenNames_WithTokens(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline", "console")
	names := e.cat.GatewayTokenNames()

	if len(names) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(names), names)
	}
	expected := []string{"ci-pipeline", "console"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected %q, got %q", name, names[i])
		}
	}
}

func TestGatewayTokenNames_NoTokens(t *testing.T) {
	e := catalogWithGatewayTokens() // no tokens
	names := e.cat.GatewayTokenNames()
	if names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestGatewayTokenNames_GatewayDisabled(t *testing.T) {
	// Gateway exists but API is disabled
	e := newExec(&catalog.Catalog{
		Gateway: &types.GatewayConfig{
			API: &types.GatewayAPIConfig{
				Enabled: false,
				Auth: types.APIAuth{
					Tokens: []types.APIToken{
						{Name: "ci-pipeline"},
					},
				},
			},
		},
	})
	names := e.cat.GatewayTokenNames()
	if names != nil {
		t.Errorf("expected nil when API disabled, got %v", names)
	}
}

func TestValidateServeTokenRestrictions_EmptyPermissionsWarning(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": {
						Permissions: types.ServePermissionSet{},
					},
				},
			},
			Warnings: types.Warnings{},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.cat.EnabledCRDs()["myresource"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for empty permissions")
	}
}

func TestValidateServeTokenRestrictions_ClusterScopedCRDWithNamespaces(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	falsePtr := false
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"clusterresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					"ci-pipeline": permsWithNamespaces(
						[]string{"get"},
						"staging",
					),
				},
			},
			Namespaced: &falsePtr,
			Warnings:   types.Warnings{},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.cat.EnabledCRDs()["clusterresource"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for namespace restrictions on cluster-scoped CRD")
	}
}

func TestValidateServeTokenRestrictions_SchemaInheritsInvalidGlobalOpWarning(t *testing.T) {
	e := catalogWithGatewayTokens("ci-pipeline")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Tokens: map[string]types.ServeTokenPermissions{
					// No explicit schema list — schema inherits from global,
					// but "create" isn't valid for schema endpoints.
					"ci-pipeline": perms("get", "create"),
				},
			},
			Warnings: types.Warnings{},
		},
	})
	if err := e.validateServeTokenRestrictions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	crd := e.cat.EnabledCRDs()["myresource"]
	if !crd.Warnings.HasWarnings() {
		t.Fatal("expected warning for global op not valid on schema endpoints")
	}
}
