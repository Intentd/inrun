package validate

import (
	"strings"
	"testing"

	"github.com/intentd/inrun/pkg/types"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

func catalogWithServeAndTarget(target string) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Enabled: true,
				Target:  types.ServeTargetValue{Shorthand: target},
			},
		},
	})
}

func catalogWithServeAndModes(target string, modes *types.ServeModes) *executor {
	return newCatalogExec(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Enabled: true,
				Target:  types.ServeTargetValue{Shorthand: target},
				Modes:   modes,
			},
		},
	})
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestValidateServeModes_NoServe(t *testing.T) {
	k := newCatalogExec(map[string]types.CRDEntry{
		"myresource": {Serve: nil},
	})
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_BothDefault(t *testing.T) {
	// Both modes default to true when Modes is nil
	k := catalogWithServeAndTarget("myapp")
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_BothExplicitTrue(t *testing.T) {
	k := catalogWithServeAndModes("myapp", &types.ServeModes{
		Target: boolPtr(true),
		CR:     boolPtr(true),
	})
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_TargetOnly(t *testing.T) {
	k := catalogWithServeAndModes("myapp", &types.ServeModes{
		Target: boolPtr(true),
		CR:     boolPtr(false),
	})
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_CROnly(t *testing.T) {
	k := catalogWithServeAndModes("", &types.ServeModes{
		Target: boolPtr(false),
		CR:     boolPtr(true),
	})
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_BothFalse(t *testing.T) {
	k := catalogWithServeAndModes("", &types.ServeModes{
		Target: boolPtr(false),
		CR:     boolPtr(false),
	})
	err := k.validateServeModes()
	if err == nil {
		t.Fatal("expected error when both modes are disabled")
	}
	if !strings.Contains(err.Error(), "at least one of serve.modes.target or serve.modes.cr must be enabled") {
		t.Errorf("error should mention both modes disabled, got: %v", err)
	}
}

func TestValidateServeModes_TargetDisabledButTargetSet(t *testing.T) {
	k := catalogWithServeAndModes("myapp", &types.ServeModes{
		Target: boolPtr(false),
		CR:     boolPtr(true),
	})
	err := k.validateServeModes()
	if err == nil {
		t.Fatal("expected error when target mode is disabled but target is set")
	}
	if !strings.Contains(err.Error(), "serve.modes.target is false but serve.target is set") {
		t.Errorf("error should mention target disabled but target set, got: %v", err)
	}
}

func TestValidateServeModes_ModesNil_WithTarget(t *testing.T) {
	k := catalogWithServeAndModes("myapp", nil)
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_ModesEmpty_WithTarget(t *testing.T) {
	// Empty Modes struct means both fields are nil → defaults to true
	k := catalogWithServeAndModes("myapp", &types.ServeModes{})
	if err := k.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
