// pkg/health/protection_provider_stats_test.go
package health

import "testing"

// ── DeletionProtectionStats ───────────────────────────────────────────────────

func TestDeletionProtectionStats_RecordBlocked(t *testing.T) {
	s := NewDeletionProtectionStats()
	s.RecordBlocked()
	snap := s.GetStats()
	if snap.TotalRequests != 1 || snap.Blocked != 1 || snap.Allowed != 0 {
		t.Errorf("unexpected snapshot after blocked: %+v", snap)
	}
}

func TestDeletionProtectionStats_RecordAllowed(t *testing.T) {
	s := NewDeletionProtectionStats()
	s.RecordAllowed()
	snap := s.GetStats()
	if snap.TotalRequests != 1 || snap.Allowed != 1 || snap.Blocked != 0 {
		t.Errorf("unexpected snapshot after allowed: %+v", snap)
	}
}

func TestDeletionProtectionStats_Mixed(t *testing.T) {
	s := NewDeletionProtectionStats()
	s.RecordBlocked()
	s.RecordBlocked()
	s.RecordAllowed()
	snap := s.GetStats()
	if snap.TotalRequests != 3 || snap.Blocked != 2 || snap.Allowed != 1 {
		t.Errorf("unexpected mixed snapshot: %+v", snap)
	}
}

func TestDeletionProtectionStats_Empty(t *testing.T) {
	s := NewDeletionProtectionStats()
	snap := s.GetStats()
	if snap.TotalRequests != 0 {
		t.Errorf("new stats must be zero, got %+v", snap)
	}
}

// ── NamespaceProtectionStats ──────────────────────────────────────────────────

func TestNamespaceProtectionStats_RecordBlocked(t *testing.T) {
	s := NewNamespaceProtectionStats()
	s.RecordBlocked()
	snap := s.GetStats()
	if snap.TotalRequests != 1 || snap.Blocked != 1 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestNamespaceProtectionStats_RecordAllowed(t *testing.T) {
	s := NewNamespaceProtectionStats()
	s.RecordAllowed()
	snap := s.GetStats()
	if snap.TotalRequests != 1 || snap.Allowed != 1 || snap.Blocked != 0 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestNamespaceProtectionStats_Mixed(t *testing.T) {
	s := NewNamespaceProtectionStats()
	s.RecordAllowed()
	s.RecordBlocked()
	snap := s.GetStats()
	if snap.TotalRequests != 2 || snap.Allowed != 1 || snap.Blocked != 1 {
		t.Errorf("unexpected mixed snapshot: %+v", snap)
	}
}
