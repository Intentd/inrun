// health/admission_stats.go
package health

import (
	"sort"
	"sync"
	"time"
)

// AdmissionStats tracks statistics for the /validate and /mutate endpoints.
// Thread-safe for concurrent updates from the admission handlers.
//
// Follows the same pattern as ConversionStats — a rolling window ring buffer
// for percentile calculations alongside simple running totals.
//
// One AdmissionStats instance lives on the HealthServer and accumulates
// statistics across all CRDs. Per-CRD breakdown is available in Prometheus
// via the crd label on the metric series.
type AdmissionStats struct {
	mu sync.RWMutex

	// ── Validation counters ──────────────────────────────────────────────────
	ValidationTotal   int64 // total /validate calls
	ValidationAllowed int64 // allowed (no deny rules fired)
	ValidationDenied  int64 // denied (at least one deny rule fired)
	ValidationWarned  int64 // allowed with warnings (warn rules fired, no deny)

	// ── Mutation counters ────────────────────────────────────────────────────
	MutationTotal   int64 // total /mutate calls
	MutationApplied int64 // at least one rule produced a change
	MutationSkipped int64 // no rules produced changes — no-op

	// ── Validation latency ───────────────────────────────────────────────────
	validationTotalLatency time.Duration
	validationMaxLatency   time.Duration
	validationMinLatency   time.Duration
	validationRing         []time.Duration
	validationRingIndex    int
	validationRingFilled   bool

	// ── Mutation latency ─────────────────────────────────────────────────────
	mutationTotalLatency time.Duration
	mutationMaxLatency   time.Duration
	mutationMinLatency   time.Duration
	mutationRing         []time.Duration
	mutationRingIndex    int
	mutationRingFilled   bool

	ringSize int
}

// NewAdmissionStats creates a new stats tracker with a rolling window.
// windowSize determines how many requests are kept for percentile calculations.
// Use the same value as ConversionStats (controlled by CONVERSION_WINDOW env var).
func NewAdmissionStats(windowSize int) *AdmissionStats {
	if windowSize <= 0 {
		windowSize = 1000
	}
	return &AdmissionStats{
		validationRing: make([]time.Duration, windowSize),
		mutationRing:   make([]time.Duration, windowSize),
		ringSize:       windowSize,
	}
}

// ── Validation recording ──────────────────────────────────────────────────────

// RecordValidationAllowed records a /validate call that was allowed with no warnings.
func (s *AdmissionStats) RecordValidationAllowed(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ValidationTotal++
	s.ValidationAllowed++
	s.recordValidationLatency(duration)
}

// RecordValidationDenied records a /validate call that was denied.
func (s *AdmissionStats) RecordValidationDenied(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ValidationTotal++
	s.ValidationDenied++
	s.recordValidationLatency(duration)
}

// RecordValidationWarned records a /validate call that was allowed with warnings.
func (s *AdmissionStats) RecordValidationWarned(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ValidationTotal++
	s.ValidationWarned++
	s.recordValidationLatency(duration)
}

// recordValidationLatency updates the validation latency tracking.
// Must be called with mu held.
func (s *AdmissionStats) recordValidationLatency(d time.Duration) {
	s.validationTotalLatency += d
	if d > s.validationMaxLatency {
		s.validationMaxLatency = d
	}
	if s.validationMinLatency == 0 || d < s.validationMinLatency {
		s.validationMinLatency = d
	}
	s.validationRing[s.validationRingIndex] = d
	s.validationRingIndex = (s.validationRingIndex + 1) % s.ringSize
	if s.validationRingIndex == 0 {
		s.validationRingFilled = true
	}
}

// ── Mutation recording ────────────────────────────────────────────────────────

// RecordMutationApplied records a /mutate call where at least one rule changed a field.
func (s *AdmissionStats) RecordMutationApplied(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MutationTotal++
	s.MutationApplied++
	s.recordMutationLatency(duration)
}

// RecordMutationSkipped records a /mutate call where no rules produced changes.
func (s *AdmissionStats) RecordMutationSkipped(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MutationTotal++
	s.MutationSkipped++
	s.recordMutationLatency(duration)
}

// recordMutationLatency updates the mutation latency tracking.
// Must be called with mu held.
func (s *AdmissionStats) recordMutationLatency(d time.Duration) {
	s.mutationTotalLatency += d
	if d > s.mutationMaxLatency {
		s.mutationMaxLatency = d
	}
	if s.mutationMinLatency == 0 || d < s.mutationMinLatency {
		s.mutationMinLatency = d
	}
	s.mutationRing[s.mutationRingIndex] = d
	s.mutationRingIndex = (s.mutationRingIndex + 1) % s.ringSize
	if s.mutationRingIndex == 0 {
		s.mutationRingFilled = true
	}
}

// ── Snapshot ──────────────────────────────────────────────────────────────────

// AdmissionStatsSnapshot is a read-only point-in-time snapshot.
// Serialised into the /blueprint/{crd} JSON response under the "admission" key.
type AdmissionStatsSnapshot struct {
	// Validation
	ValidationTotal   int64   `json:"validationTotal"`
	ValidationAllowed int64   `json:"validationAllowed"`
	ValidationDenied  int64   `json:"validationDenied"`
	ValidationWarned  int64   `json:"validationWarned"`
	ValAvgLatencyMs   float64 `json:"valAvgLatencyMs"`
	ValP95LatencyMs   float64 `json:"valP95LatencyMs"`
	ValMaxLatencyMs   float64 `json:"valMaxLatencyMs"`

	// Mutation
	MutationTotal   int64   `json:"mutationTotal"`
	MutationApplied int64   `json:"mutationApplied"`
	MutationSkipped int64   `json:"mutationSkipped"`
	MutAvgLatencyMs float64 `json:"mutAvgLatencyMs"`
	MutP95LatencyMs float64 `json:"mutP95LatencyMs"`
	MutMaxLatencyMs float64 `json:"mutMaxLatencyMs"`

	// Whether admission webhooks are enabled
	WebhooksEnabled bool `json:"webhooksEnabled"`
}

// GetStats returns a point-in-time snapshot of the admission statistics.
func (s *AdmissionStats) GetStats(webhooksEnabled bool) AdmissionStatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := AdmissionStatsSnapshot{
		ValidationTotal:   s.ValidationTotal,
		ValidationAllowed: s.ValidationAllowed,
		ValidationDenied:  s.ValidationDenied,
		ValidationWarned:  s.ValidationWarned,
		MutationTotal:     s.MutationTotal,
		MutationApplied:   s.MutationApplied,
		MutationSkipped:   s.MutationSkipped,
		WebhooksEnabled:   webhooksEnabled,
	}

	// Validation latency
	if s.ValidationTotal > 0 {
		snap.ValAvgLatencyMs = msFloat(s.validationTotalLatency / time.Duration(s.ValidationTotal))
		snap.ValMaxLatencyMs = msFloat(s.validationMaxLatency)
	}
	snap.ValP95LatencyMs = msFloat(s.percentile(s.validationRing, s.validationRingIndex, s.validationRingFilled, 0.95))

	// Mutation latency
	if s.MutationTotal > 0 {
		snap.MutAvgLatencyMs = msFloat(s.mutationTotalLatency / time.Duration(s.MutationTotal))
		snap.MutMaxLatencyMs = msFloat(s.mutationMaxLatency)
	}
	snap.MutP95LatencyMs = msFloat(s.percentile(s.mutationRing, s.mutationRingIndex, s.mutationRingFilled, 0.95))

	return snap
}

// percentile computes the nth percentile from a ring buffer.
// Identical algorithm to ConversionStats.calculatePercentile.
// Must be called with mu held (or from GetStats which holds RLock).
func (s *AdmissionStats) percentile(ring []time.Duration, index int, filled bool, p float64) time.Duration {
	size := s.ringSize
	if !filled {
		size = index
	}
	if size == 0 {
		return 0
	}
	latencies := make([]time.Duration, size)
	copy(latencies, ring[:size])
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	idx := int(float64(size) * p)
	if idx >= size {
		idx = size - 1
	}
	return latencies[idx]
}

// msFloat converts a duration to milliseconds as a float64.
// Rounds to 3 decimal places for readability.
func msFloat(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}
