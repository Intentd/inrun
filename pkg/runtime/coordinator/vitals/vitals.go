package vitals

import (
	"github.com/intentd/inrun/pkg/config"
)

// NewCRDHealth initializes a CRDHealth tracker for a given CRD name.
// The reconciler starts in an "unhealthy" state until the first successful reconcile.
func NewCRDHealth(name string) *CRDHealth {
	h := &CRDHealth{name: name}
	h.healthy.Store(false)
	h.pending.Store(true)
	h.degraded.Store(false)

	// Add catalog tracker
	h.inrunHealth = NewRuntimeHealth()
	return h
}

// NewRuntimeHealth initializes a CRDHealth tracker for Inrun
func NewRuntimeHealth() *RuntimeHealth {
	h := &RuntimeHealth{name: config.CLI}
	h.inrunReady.Store(true)
	h.katReady.Store(false)
	return h
}
