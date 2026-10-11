package vitals

import (
	"sync"
	"sync/atomic"
)

type RuntimeHealth struct {
	name       string
	inrunReady atomic.Bool
	catReady   atomic.Bool
	allOnline  atomic.Bool // For this catalog
	isLeader   atomic.Bool // true only on the pod that won the leader election
	mu         sync.RWMutex
}

// SetIsLeader marks whether this pod holds the leader election lease.
// Set to true at the start of Coordinate(), false when leadership is lost.
func (h *RuntimeHealth) SetIsLeader(v bool) {
	h.isLeader.Store(v)
}

// IsLeader reports whether this pod is the current leader (leader).
// The console uses this to decide whether to trust this pod's CRD data.
func (h *RuntimeHealth) IsLeader() bool {
	return h.isLeader.Load()
}

// SetInrunReady marks inrun engine as ready
func (h *RuntimeHealth) SetInrunReady() {
	h.inrunReady.Store(true)
}

// SetInrunDegraded marks inrun engine as degraded
func (h *RuntimeHealth) SetInrunDegraded() {
	h.inrunReady.Store(false)
}

// IsInrunReady is used to track ready state of inrun
func (h *RuntimeHealth) IsInrunReady() bool {
	return h.inrunReady.Load()
}

// SetCatalogReady marks a catalog as ready
func (h *RuntimeHealth) SetCatalogReady() {
	h.catReady.Store(true)
}

func (h *RuntimeHealth) SetRuntimeReady(ready bool) {
	h.inrunReady.Store(ready)
}

func (h *RuntimeHealth) SetAllOnline() {
	h.allOnline.Store(true)
}

func (h *RuntimeHealth) SetAllNotOnline() {
	h.allOnline.Store(false)
}

func (h *RuntimeHealth) SetReconciling(isLeader bool) {
	h.isLeader.Store(isLeader)
}

// SetCatalogDegraded marks a catalog as degraded
func (h *RuntimeHealth) SetCatalogDegraded() {
	h.catReady.Store(false)
}

// IsCatalogReady is used to track ready state of a catalog
func (h *RuntimeHealth) IsCatalogReady() bool {
	return h.catReady.Load()
}
