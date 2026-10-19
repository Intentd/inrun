package health

import (
	"strconv"
	"sync"
	"testing"
)

// Run with -race: handlers create stats while /blueprint reads them.
func TestStatsMap_ConcurrentForAndGet(t *testing.T) {
	m := NewStatsMap(NewDeletionProtectionStats)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 100 {
				key := "unknown/v1/" + strconv.Itoa((i*100+j)%10)
				m.For(key).RecordAllowed()
				_ = m.Get(key)
			}
		})
	}
	wg.Wait()

	total := int64(0)
	for i := range 10 {
		s := m.Get("unknown/v1/" + strconv.Itoa(i))
		if s == nil {
			t.Fatalf("no stats for key %d", i)
		}
		total += s.GetStats().Allowed
	}
	if total != 800 {
		t.Errorf("recorded %d, want 800", total)
	}
}

func TestStatsMap_GetDoesNotCreate(t *testing.T) {
	m := NewStatsMap(NewDeletionProtectionStats)
	if m.Get("missing") != nil {
		t.Error("Get created stats")
	}
	if m.For("k") != m.Get("k") {
		t.Error("For and Get returned different stats for the same key")
	}
}
