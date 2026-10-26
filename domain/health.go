// domain/health.go
package domain

// Health is the observability contract for any Inrun Component that
// exposes liveness and readiness state.
//
// Implementations are expected to be safe for concurrent use — all methods
// may be called from multiple goroutines simultaneously.
//
// The distinction between healthy and ready follows the Kubernetes convention:
//   - Healthy (liveness)  — the component is alive and not in a broken state.
//     An unhealthy component should be restarted.
//   - Ready (readiness)   — the component is initialised and able to serve traffic
//     or process work. A non-ready component should not receive work yet but
//     does not need to be restarted.
//
// Typical lifecycle:
//
//	created         → healthy: false, ready: false
//	Start()         → healthy: true,  ready: true   (via SetReady + Healthy)
//	Shutdown()      → healthy: false, ready: false   (via Unhealthy)
//	Error condition → healthy: false, ready: true    (via Degraded — still alive, not accepting work)
type Health interface {
	// SetReady marks the component as initialised and ready to process work.
	// Called once by the component after successful startup.
	// Implies the component is also healthy.
	SetReady()

	// Degraded marks the component as alive but not ready to process work.
	// Used when a recoverable error condition is detected — for example,
	// when a CRD's consecutive failure threshold is exceeded.
	// The component remains running but is removed from the ready pool.
	// Distinct from Unhealthy: the process does not need to be restarted.
	Degraded()

	// Unhealthy marks the component as neither healthy nor ready.
	// Called during shutdown or when an unrecoverable error is detected.
	// A Kubernetes liveness probe failing on this state will trigger a restart.
	Unhealthy()

	// Healthy returns true if the component is in a healthy state.
	// Maps to the Kubernetes liveness probe — false triggers a pod restart.
	Healthy() bool

	// Ready returns true if the component is initialised and ready to process work.
	// Maps to the Kubernetes readiness probe — false removes the pod from service endpoints.
	Ready() bool

	// Started returns true if the component has been started at least once.
	// Used by the manager to guard against double-start and to sequence
	// post-start hooks correctly.
	Started() bool
}
