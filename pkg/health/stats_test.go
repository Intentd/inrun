// pkg/health/stats_test.go
package health

import (
	"testing"
	"time"
)

// ── msFloat ───────────────────────────────────────────────────────────────────

func TestMsFloat_Zero(t *testing.T) {
	if got := msFloat(0); got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}

func TestMsFloat_OneMillisecond(t *testing.T) {
	if got := msFloat(time.Millisecond); got != 1.0 {
		t.Errorf("expected 1.0ms, got %f", got)
	}
}

func TestMsFloat_FractionalMs(t *testing.T) {
	got := msFloat(500 * time.Microsecond)
	if got != 0.5 {
		t.Errorf("expected 0.5ms, got %f", got)
	}
}

func TestMsFloat_OneSecond(t *testing.T) {
	if got := msFloat(time.Second); got != 1000.0 {
		t.Errorf("expected 1000ms, got %f", got)
	}
}

// ── AdmissionStats.percentile ─────────────────────────────────────────────────

func TestAdmissionStats_Percentile_EmptyRing_ReturnsZero(t *testing.T) {
	s := NewAdmissionStats(10)
	got := s.percentile(s.validationRing, s.validationRingIndex, s.validationRingFilled, 0.95)
	if got != 0 {
		t.Errorf("empty ring percentile must be 0, got %v", got)
	}
}

func TestAdmissionStats_Percentile_SingleEntry(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationAllowed(10 * time.Millisecond)
	snap := s.GetStats(true)
	// P95 of a single value is that value
	if snap.ValP95LatencyMs != 10.0 {
		t.Errorf("single-entry P95 must be 10ms, got %f", snap.ValP95LatencyMs)
	}
}

func TestAdmissionStats_Percentile_MultipleEntries_P95(t *testing.T) {
	s := NewAdmissionStats(100)
	// Record 100 values: 1ms, 2ms, ..., 100ms
	for i := 1; i <= 100; i++ {
		s.RecordValidationAllowed(time.Duration(i) * time.Millisecond)
	}
	snap := s.GetStats(true)
	// P95 index = int(100 * 0.95) = 95 → 95th element (0-indexed) = 96ms
	if snap.ValP95LatencyMs < 90 || snap.ValP95LatencyMs > 100 {
		t.Errorf("P95 of 1-100ms range must be near 95ms, got %f", snap.ValP95LatencyMs)
	}
}

// ── AdmissionStats counters ───────────────────────────────────────────────────

func TestAdmissionStats_RecordValidationAllowed(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationAllowed(5 * time.Millisecond)
	snap := s.GetStats(false)
	if snap.ValidationTotal != 1 || snap.ValidationAllowed != 1 {
		t.Errorf("expected total=1, allowed=1, got total=%d allowed=%d", snap.ValidationTotal, snap.ValidationAllowed)
	}
}

func TestAdmissionStats_RecordValidationDenied(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationDenied(2 * time.Millisecond)
	snap := s.GetStats(false)
	if snap.ValidationDenied != 1 || snap.ValidationTotal != 1 {
		t.Errorf("expected denied=1, got %+v", snap)
	}
}

func TestAdmissionStats_RecordValidationWarned(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationWarned(3 * time.Millisecond)
	snap := s.GetStats(false)
	if snap.ValidationWarned != 1 || snap.ValidationTotal != 1 {
		t.Errorf("expected warned=1, got %+v", snap)
	}
}

func TestAdmissionStats_RecordMutationApplied(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordMutationApplied(4 * time.Millisecond)
	snap := s.GetStats(false)
	if snap.MutationTotal != 1 || snap.MutationApplied != 1 {
		t.Errorf("expected mut total=1 applied=1, got %+v", snap)
	}
}

func TestAdmissionStats_RecordMutationSkipped(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordMutationSkipped(1 * time.Millisecond)
	snap := s.GetStats(false)
	if snap.MutationSkipped != 1 || snap.MutationTotal != 1 {
		t.Errorf("expected skipped=1, got %+v", snap)
	}
}

func TestAdmissionStats_GetStats_AvgLatency(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationAllowed(10 * time.Millisecond)
	s.RecordValidationAllowed(20 * time.Millisecond)
	snap := s.GetStats(true)
	if snap.ValAvgLatencyMs != 15.0 {
		t.Errorf("expected avg=15ms, got %f", snap.ValAvgLatencyMs)
	}
}

func TestAdmissionStats_GetStats_MaxLatency(t *testing.T) {
	s := NewAdmissionStats(10)
	s.RecordValidationAllowed(5 * time.Millisecond)
	s.RecordValidationAllowed(50 * time.Millisecond)
	snap := s.GetStats(true)
	if snap.ValMaxLatencyMs != 50.0 {
		t.Errorf("expected max=50ms, got %f", snap.ValMaxLatencyMs)
	}
}

func TestAdmissionStats_GetStats_WebhooksEnabledFlag(t *testing.T) {
	s := NewAdmissionStats(10)
	snapOn := s.GetStats(true)
	snapOff := s.GetStats(false)
	if !snapOn.WebhooksEnabled {
		t.Error("expected WebhooksEnabled=true")
	}
	if snapOff.WebhooksEnabled {
		t.Error("expected WebhooksEnabled=false")
	}
}

func TestNewAdmissionStats_ZeroWindowSize_DefaultsTo1000(t *testing.T) {
	s := NewAdmissionStats(0)
	if s.ringSize != 1000 {
		t.Errorf("expected default ringSize=1000, got %d", s.ringSize)
	}
}

// ── ConversionStats ───────────────────────────────────────────────────────────

func TestConversionStats_RecordSuccess_UpdatesCounters(t *testing.T) {
	s := NewConversionStats(10)
	s.RecordSuccess(5 * time.Millisecond)
	snap := s.GetStats()
	if snap.TotalRequests != 1 || snap.SuccessRequests != 1 {
		t.Errorf("expected total=1 success=1, got %+v", snap)
	}
}

func TestConversionStats_RecordFailure_UpdatesCounters(t *testing.T) {
	s := NewConversionStats(10)
	s.RecordFailure()
	snap := s.GetStats()
	if snap.TotalRequests != 1 || snap.FailedRequests != 1 {
		t.Errorf("expected total=1 failed=1, got %+v", snap)
	}
}

func TestConversionStats_GetStats_AvgLatency(t *testing.T) {
	s := NewConversionStats(10)
	s.RecordSuccess(10 * time.Millisecond)
	s.RecordSuccess(30 * time.Millisecond)
	snap := s.GetStats()
	if snap.AvgLatency != 20*time.Millisecond {
		t.Errorf("expected avg=20ms, got %v", snap.AvgLatency)
	}
}

func TestConversionStats_EmptyStats_ZeroLatency(t *testing.T) {
	s := NewConversionStats(10)
	snap := s.GetStats()
	if snap.AvgLatency != 0 || snap.MaxLatency != 0 {
		t.Errorf("empty stats must have zero latency, got %+v", snap)
	}
}
