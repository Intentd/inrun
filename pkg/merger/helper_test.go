// pkg/merger/helper_test.go
package merger

import (
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

func boolPtr(b bool) *bool { return &b }

// ── mergeCatalogSecurity ──────────────────────────────────────────────────────

func TestMergeCatalogSecurity_BaseWinsWhenOverrideEmpty(t *testing.T) {
	base := types.CatalogSecurity{ServiceName: &types.ServiceName{Runtime: "base-svc"}}
	override := types.CatalogSecurity{}
	result := mergeCatalogSecurity(base, override)
	if result.ServiceName == nil || result.ServiceName.Runtime != "base-svc" {
		t.Errorf("expected base ServiceName to win, got %v", result.ServiceName)
	}
}

func TestMergeCatalogSecurity_OverrideWinsServiceName(t *testing.T) {
	base := types.CatalogSecurity{ServiceName: &types.ServiceName{Runtime: "base"}}
	override := types.CatalogSecurity{ServiceName: &types.ServiceName{Runtime: "override"}}
	result := mergeCatalogSecurity(base, override)
	if result.ServiceName == nil || result.ServiceName.Runtime != "override" {
		t.Errorf("expected override ServiceName, got %v", result.ServiceName)
	}
}

func TestMergeCatalogSecurity_OverrideDeletionProtection(t *testing.T) {
	base := types.CatalogSecurity{}
	dp := &types.DeletionProtectionConfig{Enabled: boolPtr(true)}
	override := types.CatalogSecurity{DeletionProtection: dp}
	result := mergeCatalogSecurity(base, override)
	if result.DeletionProtection == nil {
		t.Error("expected override DeletionProtection to be set")
	}
}

func TestMergeCatalogSecurity_NilOverrideDeletionProtection_KeepsBase(t *testing.T) {
	dp := &types.DeletionProtectionConfig{Enabled: boolPtr(true)}
	base := types.CatalogSecurity{DeletionProtection: dp}
	override := types.CatalogSecurity{} // nil DeletionProtection
	result := mergeCatalogSecurity(base, override)
	if result.DeletionProtection == nil {
		t.Error("base DeletionProtection must be preserved when override is nil")
	}
}

func TestMergeCatalogSecurity_ServiceNameEmptyOverride_KeepsBase(t *testing.T) {
	base := types.CatalogSecurity{ServiceName: &types.ServiceName{Runtime: "my-svc"}}
	override := types.CatalogSecurity{ServiceName: nil}
	result := mergeCatalogSecurity(base, override)
	if result.ServiceName == nil || result.ServiceName.Runtime != "my-svc" {
		t.Errorf("empty override ServiceName must keep base, got %v", result.ServiceName)
	}
}

// ── mergeCatalogNotification ──────────────────────────────────────────────────

// ── checkDuplicate ────────────────────────────────────────────────────────────

func TestCheckDuplicate_FirstTime_NoError(t *testing.T) {
	seen := map[string]string{}
	err := checkDuplicate(seen, "website", "source-a.yaml")
	if err != nil {
		t.Errorf("first occurrence must not error: %v", err)
	}
}

func TestCheckDuplicate_SameSource_NoError(t *testing.T) {
	seen := map[string]string{"website": "source-a.yaml"}
	err := checkDuplicate(seen, "website", "source-a.yaml")
	if err != nil {
		t.Errorf("same source must not be a duplicate error: %v", err)
	}
}

func TestCheckDuplicate_DifferentSource_Error(t *testing.T) {
	seen := map[string]string{"website": "source-a.yaml"}
	err := checkDuplicate(seen, "website", "source-b.yaml")
	if err == nil {
		t.Error("different source must return duplicate error")
	}
}
