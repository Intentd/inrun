// pkg/health/webhook_stats.go
package health

import "sync"

// WebhookStats tracks reconciliation counters for the housekeeper.
// Thread-safe for concurrent updates from the reconciliation loop.
//
// Mirrors the pattern used by ConversionStats, AdmissionStats, and ProtectionStats.
// No latency tracking — reconciliation is periodic and the meaningful signals are
// successful cycles and failures.
type WebhookStats struct {
	mu sync.RWMutex

	Reconciled int64 // total successful reconciliation cycles
	Failed     int64 // reconciliation attempts that encountered errors
}

// NewWebhookStats creates a new WebhookStats instance.
func NewWebhookStats() *WebhookStats {
	return &WebhookStats{}
}

// RecordReconciled increments the successful reconciliation counter.
func (s *WebhookStats) RecordReconciled() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Reconciled++
}

// RecordFailure increments the reconciliation failure counter.
func (s *WebhookStats) RecordFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Failed++
}

// GetStats returns a point-in-time snapshot of webhook reconciliation statistics.
// Serialized into the /blueprint/{crd} JSON response under the "webhooks" key.
func (s *WebhookStats) GetStats() WebhookStatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return WebhookStatsSnapshot{
		Reconciled: s.Reconciled,
		Failed:     s.Failed,
	}
}

// WebhookStatsSnapshot is a read-only point-in-time snapshot.
type WebhookStatsSnapshot struct {
	Reconciled int64 `json:"reconciled"`
	Failed     int64 `json:"failed"`
}
