package serve

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/intentd/inrun/pkg/types"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

func catalogWithServeAndTarget(target string) *checker {
	return newCatalogExec(map[string]types.CRDEntry{
		"myresource": {
			Serve: &types.ServeConfig{
				Enabled: true,
				Target:  types.ServeTargetValue{Shorthand: target},
			},
		},
	})
}

func catalogWithServeAndModes(target string, modes *types.ServeModes) *checker {
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
	e := newCatalogExec(map[string]types.CRDEntry{
		"myresource": {Serve: nil},
	})
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_BothDefault(t *testing.T) {
	// Both modes default to true when Modes is nil
	e := catalogWithServeAndTarget("myapp")
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_BothExplicitTrue(t *testing.T) {
	e := catalogWithServeAndModes("myapp", &types.ServeModes{
		Target: ptr.To(true),
		CR:     ptr.To(true),
	})
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_TargetOnly(t *testing.T) {
	e := catalogWithServeAndModes("myapp", &types.ServeModes{
		Target: ptr.To(true),
		CR:     ptr.To(false),
	})
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_CROnly(t *testing.T) {
	e := catalogWithServeAndModes("", &types.ServeModes{
		Target: ptr.To(false),
		CR:     ptr.To(true),
	})
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_BothFalse(t *testing.T) {
	e := catalogWithServeAndModes("", &types.ServeModes{
		Target: ptr.To(false),
		CR:     ptr.To(false),
	})
	err := e.validateServeModes()
	if err == nil {
		t.Fatal("expected error when both modes are disabled")
	}
	if !strings.Contains(err.Error(), "at least one of serve.modes.target or serve.modes.cr must be enabled") {
		t.Errorf("error should mention both modes disabled, got: %v", err)
	}
}

func TestValidateServeModes_TargetDisabledButTargetSet(t *testing.T) {
	e := catalogWithServeAndModes("myapp", &types.ServeModes{
		Target: ptr.To(false),
		CR:     ptr.To(true),
	})
	err := e.validateServeModes()
	if err == nil {
		t.Fatal("expected error when target mode is disabled but target is set")
	}
	if !strings.Contains(err.Error(), "serve.modes.target is false but serve.target is set") {
		t.Errorf("error should mention target disabled but target set, got: %v", err)
	}
}

func TestValidateServeModes_ModesNil_WithTarget(t *testing.T) {
	e := catalogWithServeAndModes("myapp", nil)
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateServeModes_ModesEmpty_WithTarget(t *testing.T) {
	// Empty Modes struct means both fields are nil → defaults to true
	e := catalogWithServeAndModes("myapp", &types.ServeModes{})
	if err := e.validateServeModes(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
