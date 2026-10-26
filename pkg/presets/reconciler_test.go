package presets_test

import (
	"testing"
	"time"

	"github.com/intentd/inrun/pkg/presets"
	"github.com/intentd/inrun/pkg/types"
)

func TestReconcilerPresets(t *testing.T) {
	var empty *types.PresetRegistry

	tests := []struct {
		name          string
		preset        string
		expectErr     bool
		expectWorkers int
		expectResync  time.Duration
		expectDepth   int
	}{
		{"high-throughput", "high-throughput", false, 10, 5 * time.Minute, 1000},
		{"conservative", "conservative", false, 2, 1 * time.Minute, 100},
		{"development", "development", false, 1, 30 * time.Second, 50},
		{"case-insensitive HIGH-THROUGHPUT", "HIGH-THROUGHPUT", false, 10, 5 * time.Minute, 1000},
		{"case-insensitive Conservative", "Conservative", false, 2, 1 * time.Minute, 100},
		{"unknown preset fails fast", "turbo", true, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := presets.ApplyReconcilerPreset(tt.preset, empty)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Workers != tt.expectWorkers {
				t.Errorf("workers: want %d got %d", tt.expectWorkers, res.Workers)
			}
			if res.Resync != tt.expectResync {
				t.Errorf("resync: want %v got %v", tt.expectResync, res.Resync)
			}
			if res.MaxDepth != tt.expectDepth {
				t.Errorf("maxDepth: want %d got %d", tt.expectDepth, res.MaxDepth)
			}
		})
	}
}

func TestReconcilerPresetUserDefined(t *testing.T) {
	reg := types.PresetRegistry{
		Reconciler: []types.ReconcilerPresetDef{
			{
				Name:    "fast-api",
				Workers: 6,
				Resync:  types.Duration{Duration: 20 * time.Second},
				Queue:   types.Queue{MaxDepth: 300},
			},
		},
	}

	res, err := presets.ApplyReconcilerPreset("fast-api", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Workers != 6 {
		t.Errorf("workers: want 6 got %d", res.Workers)
	}
	if res.Resync != 20*time.Second {
		t.Errorf("resync: want 20s got %v", res.Resync)
	}
	if res.MaxDepth != 300 {
		t.Errorf("maxDepth: want 300 got %d", res.MaxDepth)
	}
}

func TestReconcilerPresetUserDefinedTakesPrecedenceOverBuiltIn(t *testing.T) {
	// A user-defined preset named "conservative" overrides the built-in.
	reg := types.PresetRegistry{
		Reconciler: []types.ReconcilerPresetDef{
			{
				Name:    "conservative",
				Workers: 99,
				Resync:  types.Duration{Duration: 99 * time.Second},
				Queue:   types.Queue{MaxDepth: 999},
			},
		},
	}

	res, err := presets.ApplyReconcilerPreset("conservative", &reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Workers != 99 {
		t.Errorf("user-defined should take precedence: want workers=99 got %d", res.Workers)
	}
}

func TestIsValidReconcilerPreset(t *testing.T) {
	valid := []string{"high-throughput", "conservative", "development"}
	for _, name := range valid {
		if !presets.IsValidReconcilerPreset(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	for _, name := range []string{"HIGH-THROUGHPUT", "Conservative", "DEVELOPMENT"} {
		if !presets.IsValidReconcilerPreset(name) {
			t.Errorf("expected case-insensitive %q to be valid", name)
		}
	}

	invalid := []string{"", "turbo", "fast", "slow"}
	for _, name := range invalid {
		if presets.IsValidReconcilerPreset(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
