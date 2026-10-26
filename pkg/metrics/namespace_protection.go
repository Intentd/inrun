// pkg/metrics/namespace_protection.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// namespaceProtectionBlocked
// Counts CREATE/UPDATE requests blocked by the namespace protection webhook.
// Labeled by:
//   - resource: the CRD plural resource name
//
// Alert on this metric to detect namespace policy violations:
//
//	alert: InrunNamespaceViolation
//	expr:  increase(inrun_namespace_protection_blocked_total[5m]) > 0
//
// ─────────────────────────────────────────────────────────────────────────────
var namespaceProtectionBlocked = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_namespace_protection_blocked_total",
		Help: "Number of CREATE/UPDATE requests blocked by Inrun's namespace protection webhook.",
	},
	[]string{"resource"},
)

// RecordNamespaceProtectionBlocked increments the blocked namespace protection counter.
// resource is the CRD plural name (e.g. "pipelines") of the resource that was blocked.
func RecordNamespaceProtectionBlocked(resource string) {
	namespaceProtectionBlocked.WithLabelValues(resource).Inc()
}
