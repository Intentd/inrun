package validate

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func aliasEntry(kind, target string, aliases map[string]*types.ServeTargetConfig) types.CRDEntry {
	e := serveEntry(kind, target)
	if len(aliases) > 0 {
		e = withAliases(e, aliases)
	}
	return e
}

func TestValidateServeAliases_ValidNames(t *testing.T) {
	e := catalogWithGatewayTokens("ci")
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"app": aliasEntry("App", "app", map[string]*types.ServeTargetConfig{
			"public":   {},
			"internal": {},
			"v2":       {},
		}),
	})
	if err := e.validateServeAliases(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServeAliases_InvalidName(t *testing.T) {
	e := catalogWithGatewayTokens()
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"app": aliasEntry("App", "app", map[string]*types.ServeTargetConfig{
			"Public_Alias": {}, // uppercase + underscore — invalid
		}),
	})
	err := e.validateServeAliases()
	if err == nil {
		t.Fatal("expected error for invalid alias name")
	}
	if !strings.Contains(err.Error(), "Public_Alias") {
		t.Errorf("error should mention the invalid name, got: %v", err)
	}
}

func TestValidateServeAliases_CollidesWithPrimaryTarget(t *testing.T) {
	e := catalogWithGatewayTokens()
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"app":      serveEntry("App", "app"),
		"smartapp": aliasEntry("SmartApp", "smartapp", map[string]*types.ServeTargetConfig{"app": {}}),
	})
	err := e.validateServeAliases()
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !strings.Contains(err.Error(), "app") {
		t.Errorf("error should mention colliding name: %v", err)
	}
}

func TestValidateServeAliases_CollidesWithOtherAlias(t *testing.T) {
	e := catalogWithGatewayTokens()
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"app":      aliasEntry("App", "app", map[string]*types.ServeTargetConfig{"shared": {}}),
		"smartapp": aliasEntry("SmartApp", "smartapp", map[string]*types.ServeTargetConfig{"shared": {}}),
	})
	err := e.validateServeAliases()
	if err == nil {
		t.Fatal("expected alias name collision error")
	}
}

func TestValidateServeAliases_AliasTokenMustExistInCRDTokens(t *testing.T) {
	// CRD has token restrictions — alias token not in CRD tokens.
	e := catalogWithGatewayTokens("ci", "console")
	crd := aliasEntry("App", "app", map[string]*types.ServeTargetConfig{
		"public": {
			Tokens: map[string]types.ServeTokenPermissions{
				"console": {}, // not in CRD's serve.tokens
			},
		},
	})
	crd.Serve.Tokens = map[string]types.ServeTokenPermissions{
		"ci": {}, // only ci allowed at CRD level
	}
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{"app": crd})
	err := e.validateServeAliases()
	if err == nil {
		t.Fatal("expected error: alias token not in CRD tokens")
	}
	if !strings.Contains(err.Error(), "console") {
		t.Errorf("error should mention the unknown token, got: %v", err)
	}
}

func TestValidateServeAliases_AliasTokenSubsetOfCRDTokens_Valid(t *testing.T) {
	e := catalogWithGatewayTokens("ci", "console")
	crd := aliasEntry("App", "app", map[string]*types.ServeTargetConfig{
		"public": {
			Tokens: map[string]types.ServeTokenPermissions{
				"ci": {}, // ci is in CRD tokens — valid
			},
		},
	})
	crd.Serve.Tokens = map[string]types.ServeTokenPermissions{
		"ci":      {},
		"console": {},
	}
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{"app": crd})
	if err := e.validateServeAliases(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServeAliases_NoCRDTokenRestrictions_AliasTokenCheckedAgainstGateway(t *testing.T) {
	// CRD has no token restrictions — alias can restrict to any gateway token.
	e := catalogWithGatewayTokens("ci", "console")
	crd := aliasEntry("App", "app", map[string]*types.ServeTargetConfig{
		"public": {
			Tokens: map[string]types.ServeTokenPermissions{
				"ci": {},
			},
		},
	})
	// No CRD-level tokens (all gateway tokens implicitly allowed)
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{"app": crd})
	if err := e.validateServeAliases(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServeAliases_NoCRDTokenRestrictions_UnknownGatewayToken(t *testing.T) {
	e := catalogWithGatewayTokens("ci")
	crd := aliasEntry("App", "app", map[string]*types.ServeTargetConfig{
		"public": {
			Tokens: map[string]types.ServeTokenPermissions{
				"ghost-token": {}, // not in gateway tokens
			},
		},
	})
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{"app": crd})
	err := e.validateServeAliases()
	if err == nil {
		t.Fatal("expected error: token not in gateway tokens")
	}
}

func TestValidateServeAliases_TargetDisabledNoAliases_Warns(t *testing.T) {
	e := catalogWithGatewayTokens()
	crd := serveEntry("App", "app")
	// Disable the primary surface.
	disabled := false
	crd.Serve.Target.Entries["app"].Enabled = &disabled
	// no aliases declared
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{"app": crd})
	if err := e.validateServeAliases(); err != nil {
		t.Errorf("should not error, only warn: %v", err)
	}
	// warning should have been recorded
	entry := e.cat.EnabledCRDs()["app"]
	if !entry.Warnings.HasWarnings() {
		t.Error("expected a warning for unreachable CRD")
	}
}

func TestValidateServeAliases_NoAliases_NoError(t *testing.T) {
	e := catalogWithGatewayTokens()
	e.cat.SetEnabledCRDsForTest(map[string]types.CRDEntry{
		"app": serveEntry("App", "app"),
	})
	if err := e.validateServeAliases(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServeFieldTypes(t *testing.T) {
	ok := map[string]types.ServeFieldConfig{"image": {}, "replicas": {Type: "integer"}}
	if err := validateServeFieldTypes("website", "fields", ok); err != nil {
		t.Fatalf("valid types rejected: %v", err)
	}
	bad := map[string]types.ServeFieldConfig{"replicas": {Type: "int"}}
	if err := validateServeFieldTypes("website", "fields", bad); err == nil {
		t.Fatal(`type "int" accepted`)
	}
	noEnum := map[string]types.ServeFieldConfig{"tier": {Type: "enum"}}
	if err := validateServeFieldTypes("website", "fields", noEnum); err == nil {
		t.Fatal("enum without values accepted")
	}
}
