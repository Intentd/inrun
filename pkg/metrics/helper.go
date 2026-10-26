package metrics

// ── Source constants ──────────────────────────────────────────────────────

const (
	// MetricSourceAdmission labels metrics originating from /validate or /mutate calls.
	MetricSourceAdmission = "admission"

	// MetricSourceReconcile labels metrics originating from the reconcile loop.
	MetricSourceReconcile = "reconcile"
)

// ── Recording helpers ─────────────────────────────────────────────────────
//
// Called from admission handlers and reconcile-time validation/mutation.
// All functions take the CRD name as a label — use the full GVK string
// for consistency with existing controller_reconcile_* metrics.

// RecordValidationOutcome increments the aggregate validation counter.
func RecordValidationOutcome(crd, result, source string) {
	admissionValidationTotal.WithLabelValues(crd, result, source).Inc()
}

// RecordValidationViolation increments the per-field violation counter.
func RecordValidationViolation(crd, field, rule, action, source string) {
	admissionValidationViolationsTotal.WithLabelValues(crd, field, rule, action, source).Inc()
}

// RecordMutationOutcome increments the aggregate mutation counter.
func RecordMutationOutcome(crd, result, source string) {
	admissionMutationTotal.WithLabelValues(crd, result, source).Inc()
}

// RecordMutationFieldApplied increments the per-field mutation counter.
func RecordMutationFieldApplied(crd, field, mutationType, source string) {
	admissionMutationAppliedTotal.WithLabelValues(crd, field, mutationType, source).Inc()
}

// RecordValidationDurationSeconds observes the duration of a /validate call.
// Only call this for source=admission — reconcile durations use a different histogram.
func RecordValidationDurationSeconds(crd string, seconds float64) {
	admissionValidationDuration.WithLabelValues(crd).Observe(seconds)
}

// RecordMutationDurationSeconds observes the duration of a /mutate call.
func RecordMutationDurationSeconds(crd string, seconds float64) {
	admissionMutationDuration.WithLabelValues(crd).Observe(seconds)
}

// RecordMutationTotal increments the reconcile-time mutation counter for a CRD.
func RecordMutationTotal(crd string) {
	mutationTotal.WithLabelValues(crd).Inc()
}

// RecordMutationFieldDetail increments the per-field reconcile-time mutation counter.
func RecordMutationFieldDetail(crd, field, mutationType string) {
	mutationAppliedDetail.WithLabelValues(crd, field, mutationType).Inc()
}
