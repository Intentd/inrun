// Smoke tests for pkg/metrics/metrics.go.
//
// Every public helper function is called with representative label values to
// confirm it does not panic, does not double-register a metric, and produces
// no compilation errors after the refactor from public vars to private vars +
// named helpers.
//
// These tests are intentionally not verifying counter values — Prometheus
// counter state is global and shared across the test binary. Functional
// correctness of metric recording is covered by integration tests that inspect
// the /metrics endpoint with a live Inrun instance.
package metrics_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/metrics"
	"github.com/stretchr/testify/assert"
)

// ── Reconcile metrics ─────────────────────────────────────────────────────────

func TestRecordReconcile_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordReconcile("demo.inrun.dev/v1alpha1, Kind=Website", "success")
		metrics.RecordReconcile("demo.inrun.dev/v1alpha1, Kind=Website", "failure")
	})
}

func TestObserveReconcileDuration_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.ObserveReconcileDuration("demo.inrun.dev/v1alpha1, Kind=Website", 0.042)
	})
}

func TestSetResourceCount_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.SetResourceCount("demo.inrun.dev/v1alpha1, Kind=Website", 10)
		metrics.SetResourceCount("demo.inrun.dev/v1alpha1, Kind=Website", 0)
	})
}

func TestSetQueueDepth_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.SetQueueDepth("demo.inrun.dev/v1alpha1, Kind=Website", 5)
	})
}

// ── CRD activation metrics ────────────────────────────────────────────────────

func TestObserveCRDActivationLatency_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.ObserveCRDActivationLatency("websites.demo.inrun.dev", 1.5)
	})
}

func TestRecordCRDActivation_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordCRDActivation("websites.demo.inrun.dev", "success")
		metrics.RecordCRDActivation("websites.demo.inrun.dev", "failure")
	})
}

// ── Conversion metrics ────────────────────────────────────────────────────────

func TestRecordConversion_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordConversion("Website", "v1alpha1", "v1", "success")
		metrics.RecordConversion("Website", "v1alpha1", "v1", "failure")
	})
}

func TestObserveConversionDuration_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.ObserveConversionDuration("Website", "v1alpha1", "v1", 0.0005)
	})
}

func TestRecordConversionError_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordConversionError("Website", "decode_error")
		metrics.RecordConversionError("Website", "no_path")
	})
}

func TestIncDecConversionRequests_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.IncConversionRequests()
		metrics.DecConversionRequests()
	})
}

// ── Reconcile-time mutation metrics ──────────────────────────────────────────

func TestRecordMutationTotal_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordMutationTotal("demo.inrun.dev/v1alpha1, Kind=Website")
	})
}

func TestRecordMutationFieldDetail_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordMutationFieldDetail(
			"demo.inrun.dev/v1alpha1, Kind=Website",
			"spec.replicas",
			"default",
		)
		metrics.RecordMutationFieldDetail(
			"demo.inrun.dev/v1alpha1, Kind=Website",
			"metadata.labels.managed-by",
			"override",
		)
	})
}

// RecordMutationFieldDetail is the reconcile-time variant (3-label: crd,field,type).
// RecordMutationFieldApplied is the admission-time variant (4-label: crd,field,type,source).
// Both must compile and not panic — this documents that they are distinct functions.

// ── Admission validation metrics ──────────────────────────────────────────────

func TestRecordValidationOutcome_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordValidationOutcome("Website", "allowed", metrics.MetricSourceAdmission)
		metrics.RecordValidationOutcome("Website", "denied", metrics.MetricSourceAdmission)
		metrics.RecordValidationOutcome("Website", "warned", metrics.MetricSourceReconcile)
	})
}

func TestRecordValidationViolation_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordValidationViolation("Website", "spec.image", "prefix", "deny", metrics.MetricSourceAdmission)
		metrics.RecordValidationViolation("Website", "metadata.labels.team", "exists", "warn", metrics.MetricSourceReconcile)
	})
}

func TestRecordValidationDurationSeconds_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordValidationDurationSeconds("Website", 0.0003)
	})
}

// ── Admission mutation metrics ────────────────────────────────────────────────

func TestRecordMutationOutcome_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordMutationOutcome("Website", "applied", metrics.MetricSourceAdmission)
		metrics.RecordMutationOutcome("Website", "skipped", metrics.MetricSourceReconcile)
	})
}

func TestRecordMutationFieldApplied_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordMutationFieldApplied("Website", "spec.replicas", "default", metrics.MetricSourceAdmission)
		metrics.RecordMutationFieldApplied("Website", "spec.image", "override", metrics.MetricSourceReconcile)
	})
}

func TestRecordMutationDurationSeconds_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		metrics.RecordMutationDurationSeconds("Website", 0.0002)
	})
}

// ── Source constants are defined ──────────────────────────────────────────────

func TestMetricSourceConstants(t *testing.T) {
	assert.Equal(t, "admission", metrics.MetricSourceAdmission)
	assert.Equal(t, "reconcile", metrics.MetricSourceReconcile)
}
