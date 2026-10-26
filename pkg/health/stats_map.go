package health

import "sync"

// StatsMap holds one stats value per key, such as kube.GVRKey for a CRD.
// Webhook handlers record to keys the Blueprint did not declare while
// /blueprint reads, so the map is locked; each stats value locks itself.
type StatsMap[S any] struct {
	mu       sync.RWMutex
	m        map[string]*S
	newStats func() *S
}

// NewStatsMap returns an empty StatsMap whose values newStats creates.
func NewStatsMap[S any](newStats func() *S) *StatsMap[S] {
	return &StatsMap[S]{m: map[string]*S{}, newStats: newStats}
}

// For returns the stats for key, creating them on first use.
func (s *StatsMap[S]) For(key string) *S {
	s.mu.RLock()
	v, ok := s.m[key]
	s.mu.RUnlock()
	if ok {
		return v
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[key]; ok {
		return v
	}
	v = s.newStats()
	s.m[key] = v
	return v
}

// Get returns the stats for key, or nil when nothing has created them.
func (s *StatsMap[S]) Get(key string) *S {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key]
}
