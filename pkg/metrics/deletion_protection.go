// pkg/metrics/deletion_protection.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// deletionProtectionBlocked
// Counts DELETE requests blocked by the deletion protection webhook.
// Labeled by:
//   - resource: the CRD name or "inrun-deployment"
//
// Alert on this metric to detect accidental deletion attempts:
//
//	alert: InrunDeleteAttempt
//	expr:  increase(inrun_deletion_protection_blocked_total[5m]) > 0
//
// ─────────────────────────────────────────────────────────────────────────────
var deletionProtectionBlocked = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_deletion_protection_blocked_total",
		Help: "Number of DELETE requests blocked by Inrun's deletion protection webhook.",
	},
	[]string{"resource"},
)

// RecordDeletionProtectionBlocked increments the blocked deletion counter.
// resource is the CRD full name (e.g. "pipelines.platform.io") or
// "inrun-deployment" when the Inrun deployment itself was protected.
func RecordDeletionProtectionBlocked(resource string) {
	deletionProtectionBlocked.WithLabelValues(resource).Inc()
}
