// Package domain defines the contracts the runtime, reconcilers, gateway and
// CLI share without importing each other: Reconciler and its Request and
// Result, the Object types reconcilers receive, ReconcileHooks for typed hooks,
// and Component for anything the process starts and stops.
package domain
