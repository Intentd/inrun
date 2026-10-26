// pkg/generate/validate_test.go
package generate

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// ── validateAPITypes ──────────────────────────────────────────────────────────

func fullAPITypes() types.APITypes {
	return types.APITypes{
		Object:   "Website",
		List:     "WebsiteList",
		Group:    "example.io",
		Version:  "v1",
		Kind:     "Website",
		Location: "github.com/org/repo/api/v1",
	}
}

func TestValidateAPITypes_AllFields_NoError(t *testing.T) {
	crd := types.CRDEntry{APITypes: fullAPITypes()}
	if err := validateAPITypes(crd); err != nil {
		t.Errorf("valid APITypes must not error: %v", err)
	}
}

func TestValidateAPITypes_MissingObject_Error(t *testing.T) {
	at := fullAPITypes()
	at.Object = ""
	crd := types.CRDEntry{APITypes: at}
	if err := validateAPITypes(crd); err == nil {
		t.Error("missing Object must error")
	}
}

func TestValidateAPITypes_MissingList_Error(t *testing.T) {
	at := fullAPITypes()
	at.List = ""
	crd := types.CRDEntry{APITypes: at}
	if err := validateAPITypes(crd); err == nil {
		t.Error("missing List must error")
	}
}

func TestValidateAPITypes_MissingGroup_Error(t *testing.T) {
	at := fullAPITypes()
	at.Group = ""
	crd := types.CRDEntry{APITypes: at}
	if err := validateAPITypes(crd); err == nil {
		t.Error("missing Group must error")
	}
}

func TestValidateAPITypes_MissingKind_Error(t *testing.T) {
	at := fullAPITypes()
	at.Kind = ""
	crd := types.CRDEntry{APITypes: at}
	if err := validateAPITypes(crd); err == nil {
		t.Error("missing Kind must error")
	}
}

func TestValidateAPITypes_MissingLocation_Error(t *testing.T) {
	at := fullAPITypes()
	at.Location = ""
	crd := types.CRDEntry{APITypes: at}
	if err := validateAPITypes(crd); err == nil {
		t.Error("missing Location must error")
	}
}

func TestValidateAPITypes_AllMissing_ErrorMentionsAll(t *testing.T) {
	crd := types.CRDEntry{}
	err := validateAPITypes(crd)
	if err == nil {
		t.Fatal("all-missing APITypes must error")
	}
	msg := err.Error()
	for _, field := range []string{"object", "list", "group", "version", "kind", "location"} {
		if !containsStr(msg, field) {
			t.Errorf("error must mention %q, got: %s", field, msg)
		}
	}
}

// ── validateHookEntry ─────────────────────────────────────────────────────────

func TestValidateHookEntry_BothSet_NoError(t *testing.T) {
	h := &types.HookDeclaration{Location: "github.com/org/hooks", Function: "MyHooks"}
	if err := validateHookEntry(h, "website"); err != nil {
		t.Errorf("valid hook must not error: %v", err)
	}
}

func TestValidateHookEntry_MissingLocation_Error(t *testing.T) {
	h := &types.HookDeclaration{Function: "MyHooks"}
	if err := validateHookEntry(h, "website"); err == nil {
		t.Error("missing location must error")
	}
}

func TestValidateHookEntry_MissingFunction_Error(t *testing.T) {
	h := &types.HookDeclaration{Location: "github.com/org/hooks"}
	if err := validateHookEntry(h, "website"); err == nil {
		t.Error("missing function must error")
	}
}

func TestValidateHookEntry_BothMissing_ErrorMentionsBoth(t *testing.T) {
	h := &types.HookDeclaration{}
	err := validateHookEntry(h, "website")
	if err == nil {
		t.Fatal("both-missing hook must error")
	}
	msg := err.Error()
	if !containsStr(msg, "location") || !containsStr(msg, "function") {
		t.Errorf("error must mention location and function, got: %s", msg)
	}
}

// ── validateConstructorEntry ──────────────────────────────────────────────────

func TestValidateConstructorEntry_BothSet_NoError(t *testing.T) {
	c := &types.ConstructorDeclaration{Location: "github.com/org/reconcilers", Function: "New"}
	if err := validateConstructorEntry(c, "website"); err != nil {
		t.Errorf("valid constructor must not error: %v", err)
	}
}

func TestValidateConstructorEntry_MissingLocation_Error(t *testing.T) {
	c := &types.ConstructorDeclaration{Function: "New"}
	if err := validateConstructorEntry(c, "website"); err == nil {
		t.Error("missing location must error")
	}
}

func TestValidateConstructorEntry_MissingFunction_Error(t *testing.T) {
	c := &types.ConstructorDeclaration{Location: "github.com/org/reconcilers"}
	if err := validateConstructorEntry(c, "website"); err == nil {
		t.Error("missing function must error")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
