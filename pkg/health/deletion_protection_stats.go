// pkg/health/protection_stats.go
package health

import "sync"

// DeletionProtectionStats tracks counters for the /deletion-protection webhook endpoint.
// Thread-safe for concurrent updates from the deletion protection handler.
//
// Follows the same pattern as ConversionStats, AdmissionStats, and NamespaceProtectionStats.
// No latency tracking — deletion protection decisions are fast in-memory
// lookups and the blocking count is the meaningful signal.
type DeletionProtectionStats struct {
	mu sync.RWMutex

	TotalRequests int64 // total DELETE admission reviews received
	Blocked       int64 // DELETE requests denied (CRD or deployment protected)
	Allowed       int64 // DELETE requests allowed through
}

// NewDeletionProtectionStats creates a new DeletionProtectionStats instance.
func NewDeletionProtectionStats() *DeletionProtectionStats {
	return &DeletionProtectionStats{}
}

// RecordBlocked records a DELETE that was denied by the webhook.
func (s *DeletionProtectionStats) RecordBlocked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	s.Blocked++
}

// RecordAllowed records a DELETE that was allowed through the webhook.
func (s *DeletionProtectionStats) RecordAllowed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	s.Allowed++
}

// GetStats returns a point-in-time snapshot of deletion protection statistics.
func (s *DeletionProtectionStats) GetStats() DeletionProtectionStatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return DeletionProtectionStatsSnapshot{
		TotalRequests: s.TotalRequests,
		Blocked:       s.Blocked,
		Allowed:       s.Allowed,
	}
}

// DeletionProtectionStatsSnapshot is a read-only point-in-time snapshot.
// Serialised into the /blueprint/{crd} JSON response under the "protection" key.
type DeletionProtectionStatsSnapshot struct {
	TotalRequests int64 `json:"total"`
	Blocked       int64 `json:"blocked"`
	Allowed       int64 `json:"allowed"`
}
