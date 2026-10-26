// pkg/merger/helper_test.go
package merger

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
)

// ── mergeBlueprintSecurity ──────────────────────────────────────────────────────

func TestMergeBlueprintSecurity_BaseWinsWhenOverrideEmpty(t *testing.T) {
	base := types.BlueprintSecurity{ServiceName: &types.ServiceName{Runtime: "base-svc"}}
	override := types.BlueprintSecurity{}
	result := mergeBlueprintSecurity(base, override)
	if result.ServiceName == nil || result.ServiceName.Runtime != "base-svc" {
		t.Errorf("expected base ServiceName to win, got %v", result.ServiceName)
	}
}

func TestMergeBlueprintSecurity_OverrideWinsServiceName(t *testing.T) {
	base := types.BlueprintSecurity{ServiceName: &types.ServiceName{Runtime: "base"}}
	override := types.BlueprintSecurity{ServiceName: &types.ServiceName{Runtime: "override"}}
	result := mergeBlueprintSecurity(base, override)
	if result.ServiceName == nil || result.ServiceName.Runtime != "override" {
		t.Errorf("expected override ServiceName, got %v", result.ServiceName)
	}
}

func TestMergeBlueprintSecurity_OverrideDeletionProtection(t *testing.T) {
	base := types.BlueprintSecurity{}
	dp := &types.DeletionProtectionConfig{Enabled: ptr.To(true)}
	override := types.BlueprintSecurity{DeletionProtection: dp}
	result := mergeBlueprintSecurity(base, override)
	if result.DeletionProtection == nil {
		t.Error("expected override DeletionProtection to be set")
	}
}

func TestMergeBlueprintSecurity_NilOverrideDeletionProtection_KeepsBase(t *testing.T) {
	dp := &types.DeletionProtectionConfig{Enabled: ptr.To(true)}
	base := types.BlueprintSecurity{DeletionProtection: dp}
	override := types.BlueprintSecurity{} // nil DeletionProtection
	result := mergeBlueprintSecurity(base, override)
	if result.DeletionProtection == nil {
		t.Error("base DeletionProtection must be preserved when override is nil")
	}
}

func TestMergeBlueprintSecurity_ServiceNameEmptyOverride_KeepsBase(t *testing.T) {
	base := types.BlueprintSecurity{ServiceName: &types.ServiceName{Runtime: "my-svc"}}
	override := types.BlueprintSecurity{ServiceName: nil}
	result := mergeBlueprintSecurity(base, override)
	if result.ServiceName == nil || result.ServiceName.Runtime != "my-svc" {
		t.Errorf("empty override ServiceName must keep base, got %v", result.ServiceName)
	}
}

// ── mergeBlueprintNotification ──────────────────────────────────────────────────

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
