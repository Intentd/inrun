// pkg/health/conversion_stats.go
package health

import (
	"sort"
	"sync"
	"time"
)

// ConversionStats tracks statistics for the /convert endpoint.
// Thread‑safe for concurrent updates from the conversion handler.
type ConversionStats struct {
	mu sync.RWMutex

	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64

	// For latency calculations
	totalLatency time.Duration
	maxLatency   time.Duration
	minLatency   time.Duration

	// For percentile calculations (rolling window)
	latencyRing []time.Duration
	ringIndex   int
	ringSize    int
	ringFilled  bool
}

// NewConversionStats creates a new stats tracker with a rolling window.
// windowSize determines how many requests are kept for percentile calculations.
func NewConversionStats(windowSize int) *ConversionStats {
	if windowSize <= 0 {
		windowSize = 1000 // default
	}
	return &ConversionStats{
		latencyRing: make([]time.Duration, windowSize),
		ringSize:    windowSize,
	}
}

// RecordSuccess records a successful conversion with its duration.
func (s *ConversionStats) RecordSuccess(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalRequests++
	s.SuccessRequests++

	s.totalLatency += duration
	if duration > s.maxLatency {
		s.maxLatency = duration
	}
	if s.minLatency == 0 || duration < s.minLatency {
		s.minLatency = duration
	}

	// Add to ring buffer for percentile calculation
	s.latencyRing[s.ringIndex] = duration
	s.ringIndex = (s.ringIndex + 1) % s.ringSize
	if s.ringIndex == 0 {
		s.ringFilled = true
	}
}

// RecordFailure records a failed conversion attempt.
func (s *ConversionStats) RecordFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalRequests++
	s.FailedRequests++
}

// GetStats returns a snapshot of current statistics.
func (s *ConversionStats) GetStats() ConversionStatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := ConversionStatsSnapshot{
		TotalRequests:   s.TotalRequests,
		SuccessRequests: s.SuccessRequests,
		FailedRequests:  s.FailedRequests,
		MaxLatency:      s.maxLatency,
		MinLatency:      s.minLatency,
	}

	if s.SuccessRequests > 0 {
		snapshot.AvgLatency = s.totalLatency / time.Duration(s.SuccessRequests)
	}

	// Calculate P95 from ring buffer
	snapshot.P95Latency = s.calculatePercentile(0.95)

	return snapshot
}

// calculatePercentile computes the nth percentile from the ring buffer.
func (s *ConversionStats) calculatePercentile(percentile float64) time.Duration {
	// Collect all latencies from the ring buffer
	size := s.ringSize
	if !s.ringFilled {
		size = s.ringIndex
	}
	if size == 0 {
		return 0
	}

	latencies := make([]time.Duration, size)
	for i := 0; i < size; i++ {
		latencies[i] = s.latencyRing[i]
	}

	// Sort and pick the percentile
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	idx := int(float64(size) * percentile)
	if idx >= size {
		idx = size - 1
	}
	return latencies[idx]
}

// ConversionStatsSnapshot is a read‑only snapshot of conversion statistics.
type ConversionStatsSnapshot struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	AvgLatency      time.Duration
	P95Latency      time.Duration
	MaxLatency      time.Duration
	MinLatency      time.Duration
}
