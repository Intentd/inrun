package presets

import (
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// ReconcilerPreset is a named reconciler tuning preset.
//
//   - high-throughput — workers: 10, resync: 5m, queue.maxDepth: 1000.
//     For high-volume operators processing many CRs under sustained load.
//   - conservative — workers: 2, resync: 1m, queue.maxDepth: 100.
//     Reduces API server pressure. Good default for production operators
//     where CRs change infrequently.
//   - development — workers: 1, resync: 30s, queue.maxDepth: 50.
//     Single-worker, fast resync. Makes reconcile ordering predictable
//     and logs easy to follow during local development.
type ReconcilerPreset string

const (
	ReconcilerHighThroughput ReconcilerPreset = "high-throughput"
	ReconcilerConservative   ReconcilerPreset = "conservative"
	ReconcilerDevelopment    ReconcilerPreset = "development"
)

// ReconcilerPresetResult is the expanded reconciler tuning from a named preset.
// Zero values mean "use global default" — the same semantics as omitting the field.
type ReconcilerPresetResult struct {
	Workers  int
	Resync   time.Duration
	MaxDepth int
}

// ApplyReconcilerPreset expands a named reconciler preset into its tuning values.
// User-defined presets in reg are checked first; falls back to built-ins.
// Returns an error for unknown preset names.
func ApplyReconcilerPreset(name string, reg *types.PresetRegistry) (ReconcilerPresetResult, error) {
	if reg != nil {
		if def, found := reg.LookupReconciler(name); found {
			return ReconcilerPresetResult{
				Workers:  def.Workers,
				Resync:   def.Resync.Duration,
				MaxDepth: def.Queue.MaxDepth,
			}, nil
		}
	}
	switch ReconcilerPreset(strings.ToLower(name)) {
	case ReconcilerHighThroughput:
		return ReconcilerPresetResult{Workers: 10, Resync: 5 * time.Minute, MaxDepth: 1000}, nil
	case ReconcilerConservative:
		return ReconcilerPresetResult{Workers: 2, Resync: 1 * time.Minute, MaxDepth: 100}, nil
	case ReconcilerDevelopment:
		return ReconcilerPresetResult{Workers: 1, Resync: 30 * time.Second, MaxDepth: 50}, nil
	default:
		return ReconcilerPresetResult{}, fmt.Errorf(
			"unknown reconciler preset %q — built-ins: high-throughput, conservative, development", name,
		)
	}
}

// IsValidReconcilerPreset reports whether name is a recognised built-in reconciler preset.
func IsValidReconcilerPreset(name string) bool {
	switch ReconcilerPreset(strings.ToLower(name)) {
	case ReconcilerHighThroughput, ReconcilerConservative, ReconcilerDevelopment:
		return true
	default:
		return false
	}
}
