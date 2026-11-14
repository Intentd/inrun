// Smoke tests for pkg/util/inrunmetrics/metrics.go.
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
package inrunmetrics_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/util/inrunmetrics"
	"github.com/stretchr/testify/assert"
)

// ── Reconcile metrics ─────────────────────────────────────────────────────────

func TestRecordReconcile_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordReconcile("demo.inrun.dev/v1alpha1, Kind=Website", "success")
		inrunmetrics.RecordReconcile("demo.inrun.dev/v1alpha1, Kind=Website", "failure")
	})
}

func TestObserveReconcileDuration_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.ObserveReconcileDuration("demo.inrun.dev/v1alpha1, Kind=Website", 0.042)
	})
}

func TestSetResourceCount_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.SetResourceCount("demo.inrun.dev/v1alpha1, Kind=Website", 10)
		inrunmetrics.SetResourceCount("demo.inrun.dev/v1alpha1, Kind=Website", 0)
	})
}

func TestSetQueueDepth_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.SetQueueDepth("demo.inrun.dev/v1alpha1, Kind=Website", 5)
	})
}

// ── CRD activation metrics ────────────────────────────────────────────────────

func TestObserveCRDActivationLatency_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.ObserveCRDActivationLatency("websites.demo.inrun.dev", 1.5)
	})
}

func TestRecordCRDActivation_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordCRDActivation("websites.demo.inrun.dev", "success")
		inrunmetrics.RecordCRDActivation("websites.demo.inrun.dev", "failure")
	})
}

// ── Conversion metrics ────────────────────────────────────────────────────────

func TestRecordConversion_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordConversion("Website", "v1alpha1", "v1", "success")
		inrunmetrics.RecordConversion("Website", "v1alpha1", "v1", "failure")
	})
}

func TestObserveConversionDuration_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.ObserveConversionDuration("Website", "v1alpha1", "v1", 0.0005)
	})
}

func TestRecordConversionError_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordConversionError("Website", "decode_error")
		inrunmetrics.RecordConversionError("Website", "no_path")
	})
}

func TestIncDecConversionRequests_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.IncConversionRequests()
		inrunmetrics.DecConversionRequests()
	})
}

// ── Reconcile-time mutation metrics ──────────────────────────────────────────

func TestRecordMutationTotal_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordMutationTotal("demo.inrun.dev/v1alpha1, Kind=Website")
	})
}

func TestRecordMutationFieldDetail_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordMutationFieldDetail(
			"demo.inrun.dev/v1alpha1, Kind=Website",
			"spec.replicas",
			"default",
		)
		inrunmetrics.RecordMutationFieldDetail(
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
		inrunmetrics.RecordValidationOutcome("Website", "allowed", inrunmetrics.MetricSourceAdmission)
		inrunmetrics.RecordValidationOutcome("Website", "denied", inrunmetrics.MetricSourceAdmission)
		inrunmetrics.RecordValidationOutcome("Website", "warned", inrunmetrics.MetricSourceReconcile)
	})
}

func TestRecordValidationViolation_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordValidationViolation("Website", "spec.image", "prefix", "deny", inrunmetrics.MetricSourceAdmission)
		inrunmetrics.RecordValidationViolation("Website", "metadata.labels.team", "exists", "warn", inrunmetrics.MetricSourceReconcile)
	})
}

func TestRecordValidationDurationSeconds_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordValidationDurationSeconds("Website", 0.0003)
	})
}

// ── Admission mutation metrics ────────────────────────────────────────────────

func TestRecordMutationOutcome_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordMutationOutcome("Website", "applied", inrunmetrics.MetricSourceAdmission)
		inrunmetrics.RecordMutationOutcome("Website", "skipped", inrunmetrics.MetricSourceReconcile)
	})
}

func TestRecordMutationFieldApplied_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordMutationFieldApplied("Website", "spec.replicas", "default", inrunmetrics.MetricSourceAdmission)
		inrunmetrics.RecordMutationFieldApplied("Website", "spec.image", "override", inrunmetrics.MetricSourceReconcile)
	})
}

func TestRecordMutationDurationSeconds_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		inrunmetrics.RecordMutationDurationSeconds("Website", 0.0002)
	})
}

// ── Source constants are defined ──────────────────────────────────────────────

func TestMetricSourceConstants(t *testing.T) {
	assert.Equal(t, "admission", inrunmetrics.MetricSourceAdmission)
	assert.Equal(t, "reconcile", inrunmetrics.MetricSourceReconcile)
}
