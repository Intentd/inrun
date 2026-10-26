// pkg/metrics/webhook_reconciliation.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// webhookReconciliations
// Counts successful reconciliation cycles performed by the housekeeper.
// Labeled by:
//   - type: "validating", "mutating", "deletion-protection"
//
// Use this to confirm that reconciliation is running and making progress.
//
// Example alert:
//
//	alert: InrunWebhookReconciliationStalled
//	expr:  increase(inrun_webhook_reconciliations_total[10m]) == 0
//
// ─────────────────────────────────────────────────────────────────────────────
var webhookReconciliations = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_webhook_reconciliations_total",
		Help: "Number of webhook reconciliation cycles performed by Inrun.",
	},
	[]string{"type"},
)

// RecordWebhookReconciled increments the reconciliation counter for a webhook type.
// webhookType is one of:
//   - "validating"
//   - "mutating"
//   - "deletion-protection"
func RecordWebhookReconciled(webhookType string) {
	webhookReconciliations.WithLabelValues(webhookType).Inc()
}

// ─────────────────────────────────────────────────────────────────────────────
// webhookReconciliationFailures
// Counts reconciliation failures (e.g., API errors, RBAC issues, invalid specs).
// Labeled by:
//   - type: "validating", "mutating", "deletion-protection"
//
// Alert on this metric to detect reconciliation drift or API instability.
//
//	alert: InrunWebhookReconciliationErrors
//	expr:  increase(inrun_webhook_reconciliation_failures_total[5m]) > 0
//
// ─────────────────────────────────────────────────────────────────────────────
var webhookReconciliationFailures = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_webhook_reconciliation_failures_total",
		Help: "Number of webhook reconciliation failures.",
	},
	[]string{"type"},
)

// RecordWebhookReconciliationFailure increments the failure counter for a webhook type.
func RecordWebhookReconciliationFailure(webhookType string) {
	webhookReconciliationFailures.WithLabelValues(webhookType).Inc()
}
