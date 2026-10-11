package profiles

import (
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// ReconcilerProfile is a named reconciler tuning preset.
//
//   - high-throughput — workers: 10, resync: 5m, queue.maxDepth: 1000.
//     For high-volume operators processing many CRs under sustained load.
//   - conservative — workers: 2, resync: 1m, queue.maxDepth: 100.
//     Reduces API server pressure. Good default for production operators
//     where CRs change infrequently.
//   - development — workers: 1, resync: 30s, queue.maxDepth: 50.
//     Single-worker, fast resync. Makes reconcile ordering predictable
//     and logs easy to follow during local development.
type ReconcilerProfile string

const (
	ReconcilerHighThroughput ReconcilerProfile = "high-throughput"
	ReconcilerConservative   ReconcilerProfile = "conservative"
	ReconcilerDevelopment    ReconcilerProfile = "development"
)

// ReconcilerProfileResult is the expanded reconciler tuning from a named profile.
// Zero values mean "use global default" — the same semantics as omitting the field.
type ReconcilerProfileResult struct {
	Workers  int
	Resync   time.Duration
	MaxDepth int
}

// ApplyReconcilerProfile expands a named reconciler profile into its tuning values.
// User-defined profiles in reg are checked first; falls back to built-ins.
// Returns an error for unknown profile names.
func ApplyReconcilerProfile(name string, reg *types.ProfileRegistry) (ReconcilerProfileResult, error) {
	if reg != nil {
		if def, found := reg.LookupReconciler(name); found {
			return ReconcilerProfileResult{
				Workers:  def.Workers,
				Resync:   def.Resync.Duration,
				MaxDepth: def.Queue.MaxDepth,
			}, nil
		}
	}
	switch ReconcilerProfile(strings.ToLower(name)) {
	case ReconcilerHighThroughput:
		return ReconcilerProfileResult{Workers: 10, Resync: 5 * time.Minute, MaxDepth: 1000}, nil
	case ReconcilerConservative:
		return ReconcilerProfileResult{Workers: 2, Resync: 1 * time.Minute, MaxDepth: 100}, nil
	case ReconcilerDevelopment:
		return ReconcilerProfileResult{Workers: 1, Resync: 30 * time.Second, MaxDepth: 50}, nil
	default:
		return ReconcilerProfileResult{}, fmt.Errorf(
			"unknown reconciler profile %q — built-ins: high-throughput, conservative, development", name,
		)
	}
}

// IsValidReconcilerProfile reports whether name is a recognised built-in reconciler profile.
func IsValidReconcilerProfile(name string) bool {
	switch ReconcilerProfile(strings.ToLower(name)) {
	case ReconcilerHighThroughput, ReconcilerConservative, ReconcilerDevelopment:
		return true
	default:
		return false
	}
}
