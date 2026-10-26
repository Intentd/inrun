// Tests for admission_stats.go — the rolling-window statistics tracker.
//
// Package health (white-box) — accesses unexported ring buffer fields
// to verify the internal latency accounting state directly.
package health

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── Constructor ───────────────────────────────────────────────────────────────

func TestNewAdmissionStats_DefaultWindowSize(t *testing.T) {
	s := NewAdmissionStats(0) // 0 triggers the default
	assert.Equal(t, 1000, s.ringSize)
	assert.Len(t, s.validationRing, 1000)
	assert.Len(t, s.mutationRing, 1000)
}

func TestNewAdmissionStats_NegativeWindowSize(t *testing.T) {
	s := NewAdmissionStats(-5) // negative also triggers default
	assert.Equal(t, 1000, s.ringSize)
}

func TestNewAdmissionStats_CustomWindowSize(t *testing.T) {
	s := NewAdmissionStats(50)
	assert.Equal(t, 50, s.ringSize)
	assert.Len(t, s.validationRing, 50)
	assert.Len(t, s.mutationRing, 50)
}

// ── Validation counter recording ──────────────────────────────────────────────

func TestRecordValidationAllowed_IncrementsCorrectCounters(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationAllowed(1 * time.Millisecond)

	assert.Equal(t, int64(1), s.ValidationTotal)
	assert.Equal(t, int64(1), s.ValidationAllowed)
	assert.Equal(t, int64(0), s.ValidationDenied)
	assert.Equal(t, int64(0), s.ValidationWarned)
}

func TestRecordValidationDenied_IncrementsCorrectCounters(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationDenied(2 * time.Millisecond)

	assert.Equal(t, int64(1), s.ValidationTotal)
	assert.Equal(t, int64(0), s.ValidationAllowed)
	assert.Equal(t, int64(1), s.ValidationDenied)
	assert.Equal(t, int64(0), s.ValidationWarned)
}

func TestRecordValidationWarned_IncrementsCorrectCounters(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationWarned(3 * time.Millisecond)

	assert.Equal(t, int64(1), s.ValidationTotal)
	assert.Equal(t, int64(0), s.ValidationAllowed)
	assert.Equal(t, int64(0), s.ValidationDenied)
	assert.Equal(t, int64(1), s.ValidationWarned)
}

// ── Mutation counter recording ────────────────────────────────────────────────

func TestRecordMutationApplied_IncrementsCorrectCounters(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordMutationApplied(4 * time.Millisecond)

	assert.Equal(t, int64(1), s.MutationTotal)
	assert.Equal(t, int64(1), s.MutationApplied)
	assert.Equal(t, int64(0), s.MutationSkipped)
}

func TestRecordMutationSkipped_IncrementsCorrectCounters(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordMutationSkipped(1 * time.Millisecond)

	assert.Equal(t, int64(1), s.MutationTotal)
	assert.Equal(t, int64(0), s.MutationApplied)
	assert.Equal(t, int64(1), s.MutationSkipped)
}

// ── Mixed records: totals stay consistent ─────────────────────────────────────

func TestAdmissionStats_AccumulatesCorrectly(t *testing.T) {
	s := NewAdmissionStats(100)

	s.RecordValidationAllowed(1 * time.Millisecond)
	s.RecordValidationAllowed(2 * time.Millisecond)
	s.RecordValidationDenied(5 * time.Millisecond)
	s.RecordValidationWarned(3 * time.Millisecond)
	s.RecordMutationApplied(4 * time.Millisecond)
	s.RecordMutationSkipped(1 * time.Millisecond)

	assert.Equal(t, int64(4), s.ValidationTotal)
	assert.Equal(t, int64(2), s.ValidationAllowed)
	assert.Equal(t, int64(1), s.ValidationDenied)
	assert.Equal(t, int64(1), s.ValidationWarned)
	assert.Equal(t, int64(2), s.MutationTotal)
	assert.Equal(t, int64(1), s.MutationApplied)
	assert.Equal(t, int64(1), s.MutationSkipped)
}

// ── GetStats snapshot ─────────────────────────────────────────────────────────

func TestGetStats_CountersReflectedInSnapshot(t *testing.T) {
	s := NewAdmissionStats(100)

	s.RecordValidationAllowed(1 * time.Millisecond)
	s.RecordValidationDenied(2 * time.Millisecond)
	s.RecordMutationApplied(3 * time.Millisecond)

	snap := s.GetStats(true)

	assert.Equal(t, int64(2), snap.ValidationTotal)
	assert.Equal(t, int64(1), snap.ValidationAllowed)
	assert.Equal(t, int64(1), snap.ValidationDenied)
	assert.Equal(t, int64(0), snap.ValidationWarned)
	assert.Equal(t, int64(1), snap.MutationTotal)
	assert.Equal(t, int64(1), snap.MutationApplied)
	assert.Equal(t, int64(0), snap.MutationSkipped)
	assert.True(t, snap.WebhooksEnabled)
}

func TestGetStats_WebhooksEnabledFlag(t *testing.T) {
	s := NewAdmissionStats(10)

	snapEnabled := s.GetStats(true)
	snapDisabled := s.GetStats(false)

	assert.True(t, snapEnabled.WebhooksEnabled)
	assert.False(t, snapDisabled.WebhooksEnabled)
}

func TestGetStats_ZeroLatencyWhenNoRecords(t *testing.T) {
	s := NewAdmissionStats(10)
	snap := s.GetStats(false)

	assert.Equal(t, float64(0), snap.ValAvgLatencyMs)
	assert.Equal(t, float64(0), snap.ValMaxLatencyMs)
	assert.Equal(t, float64(0), snap.ValP95LatencyMs)
	assert.Equal(t, float64(0), snap.MutAvgLatencyMs)
	assert.Equal(t, float64(0), snap.MutMaxLatencyMs)
	assert.Equal(t, float64(0), snap.MutP95LatencyMs)
}

// ── Latency calculations ──────────────────────────────────────────────────────

func TestGetStats_AvgValidationLatency(t *testing.T) {
	s := NewAdmissionStats(100)
	s.RecordValidationAllowed(2 * time.Millisecond)
	s.RecordValidationAllowed(4 * time.Millisecond)

	snap := s.GetStats(true)

	// avg = (2+4)/2 = 3 ms
	assert.InDelta(t, 3.0, snap.ValAvgLatencyMs, 0.5)
}

func TestGetStats_MaxValidationLatency(t *testing.T) {
	s := NewAdmissionStats(100)
	s.RecordValidationAllowed(1 * time.Millisecond)
	s.RecordValidationAllowed(5 * time.Millisecond)
	s.RecordValidationAllowed(2 * time.Millisecond)

	snap := s.GetStats(true)

	assert.InDelta(t, 5.0, snap.ValMaxLatencyMs, 0.5)
}

func TestGetStats_MinLatencyTracked(t *testing.T) {
	s := NewAdmissionStats(100)
	s.RecordValidationAllowed(10 * time.Millisecond)
	s.RecordValidationAllowed(3 * time.Millisecond)
	s.RecordValidationAllowed(7 * time.Millisecond)

	// min is 3ms — used internally for tracking, not surfaced in snapshot
	// but we verify through avg and max that all records are counted
	snap := s.GetStats(true)
	assert.InDelta(t, 10.0, snap.ValMaxLatencyMs, 0.5)
	assert.InDelta(t, (10.0+3.0+7.0)/3, snap.ValAvgLatencyMs, 0.5)
}

func TestGetStats_MutationLatency(t *testing.T) {
	s := NewAdmissionStats(100)
	s.RecordMutationApplied(6 * time.Millisecond)
	s.RecordMutationSkipped(2 * time.Millisecond)

	snap := s.GetStats(true)

	assert.InDelta(t, 4.0, snap.MutAvgLatencyMs, 0.5) // (6+2)/2
	assert.InDelta(t, 6.0, snap.MutMaxLatencyMs, 0.5)
}

// ── Ring buffer / percentile ──────────────────────────────────────────────────

func TestGetStats_P95_Ascending(t *testing.T) {
	s := NewAdmissionStats(100)

	// Record 20 latencies: 1ms to 20ms
	for i := 1; i <= 20; i++ {
		s.RecordValidationAllowed(time.Duration(i) * time.Millisecond)
	}

	snap := s.GetStats(true)
	// P95 of [1..20]: index = int(20*0.95) = 19 → 20ms
	assert.InDelta(t, 20.0, snap.ValP95LatencyMs, 1.0)
}

func TestGetStats_P95_WithSmallWindow(t *testing.T) {
	// Ring size 5 — only the last 5 writes are visible for percentile
	s := NewAdmissionStats(5)

	// Write 10 records; the ring wraps and holds [6..10]
	for i := 1; i <= 10; i++ {
		s.RecordValidationAllowed(time.Duration(i) * time.Millisecond)
	}

	snap := s.GetStats(true)
	// All 10 are counted in ValidationTotal
	assert.Equal(t, int64(10), snap.ValidationTotal)
	// P95 is computed from ring [6,7,8,9,10] → sorted: p95 index 4 → 10ms
	assert.InDelta(t, 10.0, snap.ValP95LatencyMs, 1.0)
}

func TestGetStats_P95_SingleRecord(t *testing.T) {
	s := NewAdmissionStats(100)
	s.RecordValidationAllowed(7 * time.Millisecond)

	snap := s.GetStats(true)
	assert.InDelta(t, 7.0, snap.ValP95LatencyMs, 0.5)
}

// ── msFloat helper ────────────────────────────────────────────────────────────

func TestMsFloat(t *testing.T) {
	assert.InDelta(t, 1.0, msFloat(1*time.Millisecond), 0.001)
	assert.InDelta(t, 0.5, msFloat(500*time.Microsecond), 0.001)
	assert.InDelta(t, 0.0, msFloat(0), 0.001)
}

// ── Concurrency safety ────────────────────────────────────────────────────────

func TestAdmissionStats_ConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test in short mode")
	}

	s := NewAdmissionStats(1000)

	const goroutines = 20
	const recordsEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < recordsEach; j++ {
				s.RecordValidationAllowed(time.Millisecond)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < recordsEach; j++ {
				s.RecordValidationDenied(2 * time.Millisecond)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < recordsEach; j++ {
				s.RecordMutationApplied(time.Millisecond)
				// interleave reads with writes
				_ = s.GetStats(true)
			}
		}()
	}

	wg.Wait()

	snap := s.GetStats(true)
	assert.Equal(t, int64(goroutines*recordsEach*2), snap.ValidationTotal)
	assert.Equal(t, int64(goroutines*recordsEach), snap.ValidationAllowed)
	assert.Equal(t, int64(goroutines*recordsEach), snap.ValidationDenied)
	assert.Equal(t, int64(goroutines*recordsEach), snap.MutationTotal)
	assert.Equal(t, int64(goroutines*recordsEach), snap.MutationApplied)
}

// ── Snapshot is a copy: mutations after GetStats don't affect the snapshot ────

func TestGetStats_SnapshotIsIsolatedFromFutureWrites(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationAllowed(1 * time.Millisecond)

	snap := s.GetStats(true)
	assert.Equal(t, int64(1), snap.ValidationTotal)

	// Write more records after snapshot
	s.RecordValidationDenied(2 * time.Millisecond)
	s.RecordValidationDenied(3 * time.Millisecond)

	// Snapshot is unchanged
	assert.Equal(t, int64(1), snap.ValidationTotal)

	// New snapshot reflects new writes
	snap2 := s.GetStats(true)
	assert.Equal(t, int64(3), snap2.ValidationTotal)
}
