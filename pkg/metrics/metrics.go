package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// reconcileTotal
// Counts every reconcile attempt for every CRD.
// Labeled by:
//   - crd: CRD name
//   - result: "success" or "failure"
//
// This is the primary volume metric for operator activity.
// ─────────────────────────────────────────────────────────────────────────────
var reconcileTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "controller_reconcile_total",
	Help: "Total number of reconciliations",
}, []string{"crd", "result"})

// RecordReconcile increments the reconcile counter for a CRD.
func RecordReconcile(gvk, result string) {
	reconcileTotal.WithLabelValues(gvk, result).Inc()
}

// ─────────────────────────────────────────────────────────────────────────────
// reconcileDuration
// Histogram of reconcile latency per CRD.
// Helps identify slow reconcilers, API bottlenecks, or template evaluation cost.
// ─────────────────────────────────────────────────────────────────────────────
var reconcileDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "controller_reconcile_duration_seconds",
	Help:    "Duration of reconciliations",
	Buckets: prometheus.DefBuckets,
}, []string{"crd"})

// ObserveReconcileDuration records a reconcile duration observation.
func ObserveReconcileDuration(gvk string, seconds float64) {
	reconcileDuration.WithLabelValues(gvk).Observe(seconds)
}

// ─────────────────────────────────────────────────────────────────────────────
// resourceCount
// Gauge of how many CRs exist for each CRD (from informer cache).
// Useful for:
//   - capacity planning
//   - detecting runaway CR creation
//   - validating informer correctness
//
// ─────────────────────────────────────────────────────────────────────────────
var resourceCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "controller_resource_count",
	Help: "Number of custom resources (CR) per CRD",
}, []string{"crd"})

// SetResourceCount sets the resource count gauge for a CRD.
func SetResourceCount(gvk string, count float64) {
	resourceCount.WithLabelValues(gvk).Set(count)
}

// ─────────────────────────────────────────────────────────────────────────────
// queueDepth
// Gauge of the current work queue depth per CRD.
// High queue depth indicates backpressure or insufficient workers.
// ─────────────────────────────────────────────────────────────────────────────
var queueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "controller_queue_depth",
	Help: "Current queue depth per CRD",
}, []string{"crd"})

// SetQueueDepth sets the queue depth gauge for a CRD.
func SetQueueDepth(gvk string, depth float64) {
	queueDepth.WithLabelValues(gvk).Set(depth)
}

// ─────────────────────────────────────────────────────────────────────────────
// workersActive
// Gauge of how many workers are actively reconciling per CRD.
// Helps tune worker counts and detect worker starvation.
// ─────────────────────────────────────────────────────────────────────────────
var workersTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "controller_workers_total",
	Help: "Total number of workers per CRD",
}, []string{"crd"})

var workersIdle = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "controller_workers_idle",
	Help: "Number of idle workers per CRD",
}, []string{"crd"})

var workersProcessing = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "controller_workers_processing",
	Help: "Number of processing workers per CRD",
}, []string{"crd"})

// SetWorkersActive sets the active worker gauge for a CRD.
// func SetWorkersActive(gvk string, count float64) {
// 	workersActive.WithLabelValues(gvk).Set(count)
// }

func SetWorkersTotal(gvk string, value float64) {
	workersTotal.WithLabelValues(gvk).Set(value)
}

func SetWorkersProcessing(gvk string, value float64) {
	workersProcessing.WithLabelValues(gvk).Set(value)
}

func SetWorkersIdle(gvk string, value float64) {
	workersIdle.WithLabelValues(gvk).Set(value)
}

// ─────────────────────────────────────────────────────────────────────────────
// crdActivationLatency
// Histogram measuring how long it takes for a CRD to become "active"
// after the operator starts (i.e., informers synced + reconciler ready).
// Useful for startup diagnostics and readiness tuning.
// ─────────────────────────────────────────────────────────────────────────────
var crdActivationLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "controller_crd_activation_latency_seconds",
	Help:    "Time from controller start to CRD becoming active",
	Buckets: prometheus.DefBuckets,
}, []string{"crd"})

// ObserveCRDActivationLatency records how long a CRD took to become active.
func ObserveCRDActivationLatency(crd string, seconds float64) {
	crdActivationLatency.WithLabelValues(crd).Observe(seconds)
}

// ─────────────────────────────────────────────────────────────────────────────
// crdActivationTotal
// Counts CRD activation attempts after startup.
// Labeled by:
//   - crd
//   - result: "success" or "failure"
//
// Helps detect CRDs that repeatedly fail to initialize.
// ─────────────────────────────────────────────────────────────────────────────
var crdActivationTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "controller_crd_activation_total",
	Help: "Total number of CRD activations after startup",
}, []string{"crd", "result"})

// RecordCRDActivation increments the CRD activation counter.
func RecordCRDActivation(crd, result string) {
	crdActivationTotal.WithLabelValues(crd, result).Inc()
}

// ─────────────────────────────────────────────────────────────────────────────
// conversionTotal
// Counts every conversion request.
// Labeled by:
//   - kind: CRD kind (e.g., "Website")
//   - from_version: source API version (e.g., "v1alpha1")
//   - to_version: target API version (e.g., "v1")
//   - result: "success" or "failure"
//
// ─────────────────────────────────────────────────────────────────────────────
var conversionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "inrun_conversion_requests_total",
	Help: "Total number of conversion requests processed",
}, []string{"kind", "from_version", "to_version", "result"})

// RecordConversion increments the conversion request counter.
func RecordConversion(kind, fromVersion, toVersion, result string) {
	conversionTotal.WithLabelValues(kind, fromVersion, toVersion, result).Inc()
}

// ─────────────────────────────────────────────────────────────────────────────
// conversionDuration
// Histogram of conversion latency per kind and direction.
// Helps identify slow conversions that might affect API server performance.
// ─────────────────────────────────────────────────────────────────────────────
var conversionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "inrun_conversion_duration_seconds",
	Help:    "Duration of conversion requests",
	Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
}, []string{"kind", "from_version", "to_version"})

// ObserveConversionDuration records a conversion duration observation.
func ObserveConversionDuration(kind, fromVersion, toVersion string, seconds float64) {
	conversionDuration.WithLabelValues(kind, fromVersion, toVersion).Observe(seconds)
}

// ─────────────────────────────────────────────────────────────────────────────
// conversionErrors
// Counts conversion errors by kind and error type.
// Helps debug conversion rule misconfigurations.
// ─────────────────────────────────────────────────────────────────────────────
var conversionErrors = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "inrun_conversion_errors_total",
	Help: "Total number of conversion errors",
}, []string{"kind", "error_type"})

// RecordConversionError increments the conversion error counter.
func RecordConversionError(kind, errorType string) {
	conversionErrors.WithLabelValues(kind, errorType).Inc()
}

// ─────────────────────────────────────────────────────────────────────────────
// conversionActiveRequests
// Gauge of currently in‑flight conversion requests.
// Helps detect conversion backpressure.
// ─────────────────────────────────────────────────────────────────────────────
var conversionActiveRequests = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "inrun_conversion_active_requests",
	Help: "Number of conversion requests currently being processed",
})

// IncConversionRequests increments the in-flight conversion request gauge.
func IncConversionRequests() {
	conversionActiveRequests.Inc()
}

// DecConversionRequests decrements the in-flight conversion request gauge.
func DecConversionRequests() {
	conversionActiveRequests.Dec()
}

// ── Prometheus metrics for admission webhooks ──────────────────────────────
//
// Four metrics. Each earns its place.
//
// ── controller_admission_validation_total ─────────────────────────────────
//
//   Labels: crd, result, source
//
//   result: "allowed" | "denied" | "warned"
//   source: "admission"  — call from /validate endpoint (kubectl apply time)
//           "reconcile"  — call from the reconcile loop
//
//   The source label is the operationally critical dimension.
//   A high "denied" count with source="reconcile" and source="admission" means
//   the deny rule is working at both points.
//   A high "denied" count with source="reconcile" only means ENABLE_ADMISSION_WEBHOOK
//   is not set or the webhook is not intercepting — CRs are slipping through.
//
//   Alert example:
//     rate(controller_admission_validation_total{result="denied",source="reconcile"}[5m]) > 0
//     AND rate(controller_admission_validation_total{result="denied",source="admission"}[5m]) == 0
//     → CRs are failing validation at reconcile time but not being caught at apply time
//     → Check ENABLE_ADMISSION_WEBHOOK and webhook configuration

var admissionValidationTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_admission_validation_total",
		Help: "Total validation checks by CRD, result (allowed|denied|warned), and source (admission|reconcile).",
	},
	[]string{"crd", "result", "source"},
)

// ── controller_admission_validation_violations_total ──────────────────────
//
//   Labels: crd, field, rule, action, source
//
//   Counts individual rule violations — not aggregate outcomes.
//   Use this to understand which specific rules are firing most, and whether
//   they are firing at admission time or only at reconcile time.
//
//   action: "deny" | "warn"
//   source: "admission" | "reconcile"
//
//   Example queries:
//     # Which fields are denied most often at admission time?
//     topk(5, sum by(field) (
//       controller_admission_validation_violations_total{action="deny",source="admission"}
//     ))
//
//     # Which warn rules are never firing at admission (ENABLE_ADMISSION_WEBHOOK=false)?
//     sum by(field) (
//       controller_admission_validation_violations_total{action="warn",source="reconcile"}
//     )
//     UNLESS
//     sum by(field) (
//       controller_admission_validation_violations_total{action="warn",source="admission"}
//     ) > 0

var admissionValidationViolationsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_admission_validation_violations_total",
		Help: "Validation rule violations by CRD, field, rule type, action, and source.",
	},
	[]string{"crd", "field", "rule", "action", "source"},
)

// ── controller_admission_mutation_total ───────────────────────────────────
//
//   Labels: crd, result, source
//
//   result: "applied"  — at least one rule changed a field value
//           "skipped"  — no rules produced changes (all fields already set)
//
//   source: "admission" | "reconcile"
//
//   High "applied" at source="admission" means users frequently omit required
//   fields — a signal that client tooling or CRD documentation should improve.
//
//   High "applied" at source="reconcile" only (with source="admission" = 0)
//   means ENABLE_ADMISSION_WEBHOOK is not set — defaults are being applied late,
//   after the CR is stored without them.

var admissionMutationTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_admission_mutation_total",
		Help: "Total mutation evaluations by CRD, result (applied|skipped), and source (admission|reconcile).",
	},
	[]string{"crd", "result", "source"},
)

// ── controller_admission_mutation_applied_total ───────────────────────────
//
//   Labels: crd, field, type, source
//
//   type: "default"  — field was absent, default value was set
//         "override" — field was overridden regardless of current value
//
//   source: "admission" | "reconcile"
//
//   Use this to understand which specific fields are being defaulted most often.
//   High "default" count for a field → users are not setting it, defaults carry the load.
//   High "override" count for a field → users are setting it but platform is correcting it.
//
//   Example query:
//     # Which fields are most often absent at admission time?
//     topk(10, sum by(field) (
//       controller_admission_mutation_applied_total{type="default",source="admission"}
//     ))

var admissionMutationAppliedTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_admission_mutation_applied_total",
		Help: "Individual field mutations applied, by CRD, field path, mutation type, and source.",
	},
	[]string{"crd", "field", "type", "source"},
)

// ── Duration histograms ───────────────────────────────────────────────────
//
// Two histograms — one for /validate, one for /mutate.
// Admission-time only (source="admission"). Reconcile-time validation and
// mutation are measured by the existing controller_reconcile_duration_seconds.
//
// Buckets: sub-millisecond to 100ms. Admission webhook timeout is 5 seconds.
// Rule evaluation should complete in under 1ms for typical Blueprint sizes.
// Alert if p99 exceeds 50ms — that is approaching timeout territory.

var admissionValidationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "controller_admission_validation_duration_seconds",
		Help:    "Duration of /validate endpoint calls. Only recorded for source=admission.",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
	},
	[]string{"crd"},
)

var admissionMutationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "controller_admission_mutation_duration_seconds",
		Help:    "Duration of /mutate endpoint calls. Only recorded for source=admission.",
		Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
	},
	[]string{"crd"},
)

// ─────────────────────────────────────────────────────────────────────────────
// mutationTotal
// Counts reconciles where at least one mutation/defaulting rule was applied.
// Helps understand how often CRs rely on defaults or auto‑correction.
// ─────────────────────────────────────────────────────────────────────────────
var mutationTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_mutation_total",
		Help: "Total reconciles where at least one mutation rule was applied.",
	},
	[]string{"crd"},
)

// ─────────────────────────────────────────────────────────────────────────────
// mutationAppliedDetail
// Counts individual field‑level mutations.
// Labeled by:
//   - crd
//   - field: the field mutated
//   - type: mutation type (default, override, normalize, etc.)
//
// Helps identify:
//   - which fields are most often missing
//   - which defaults are most active
//   - where schema drift or user misconfiguration occurs
//
// ─────────────────────────────────────────────────────────────────────────────
var mutationAppliedDetail = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_mutation_applied_total",
		Help: "Mutations applied per field, labeled by CRD, field, and mutation type.",
	},
	[]string{"crd", "field", "type"},
)
