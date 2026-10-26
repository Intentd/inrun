// pkg/health/namespace_protection_stats.go
//
// NamespaceProtectionStats tracks counters for the /namespace-protection webhook.
// Distinct from DeletionProtectionStats (deletion protection) — different admission operations,
// different semantics. Deletion protection intercepts DELETE; namespace protection
// intercepts CREATE and UPDATE.
package health

import "sync"

// NamespaceProtectionStats tracks counters for the /namespace-protection webhook endpoint.
// Thread-safe for concurrent updates from the namespace protection handler.
type NamespaceProtectionStats struct {
	mu sync.RWMutex

	TotalRequests int64 // total CREATE/UPDATE admission reviews received
	Blocked       int64 // requests denied — namespace not in allowed set / in restricted set
	Allowed       int64 // requests allowed through
}

// NewNamespaceProtectionStats creates a new NamespaceProtectionStats instance.
func NewNamespaceProtectionStats() *NamespaceProtectionStats {
	return &NamespaceProtectionStats{}
}

// RecordBlocked records a CREATE/UPDATE that was denied by the namespace rule.
func (s *NamespaceProtectionStats) RecordBlocked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	s.Blocked++
}

// RecordAllowed records a CREATE/UPDATE that passed the namespace rule.
func (s *NamespaceProtectionStats) RecordAllowed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	s.Allowed++
}

// GetStats returns a point-in-time snapshot of namespace protection statistics.
func (s *NamespaceProtectionStats) GetStats() NamespaceProtectionStatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return NamespaceProtectionStatsSnapshot{
		TotalRequests: s.TotalRequests,
		Blocked:       s.Blocked,
		Allowed:       s.Allowed,
	}
}

// NamespaceProtectionStatsSnapshot is a read-only point-in-time snapshot.
// Serialised into the /blueprint/{crd} JSON response under the "namespaceProtection" key.
type NamespaceProtectionStatsSnapshot struct {
	TotalRequests int64 `json:"total"`
	Blocked       int64 `json:"blocked"`
	Allowed       int64 `json:"allowed"`
}
