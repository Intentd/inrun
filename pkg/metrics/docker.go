package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// Docker operation metrics
//
// Tracks docker build/push operations executed by the reconciler.
//
// Labels:
//   - crd:       CRD name (GVK string)
//   - operation: "build" | "push"
//   - image:     image reference
//   - result:    "success" | "error"
// ─────────────────────────────────────────────────────────────────────────────

var dockerOperationsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_docker_operations_total",
		Help: "Total number of Docker operations performed by the reconciler.",
	},
	[]string{"crd", "operation", "image", "result"},
)

var dockerOperationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "inrun_docker_operation_duration_seconds",
		Help:    "Duration of Docker operations.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"crd", "operation", "image"},
)

var dockerOperationErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_docker_operation_errors_total",
		Help: "Total number of Docker operation errors, labelled by error type.",
	},
	[]string{"crd", "operation", "image", "error_type"},
)

// RecordDockerOperation records a Docker operation result and duration.
func RecordDockerOperation(crd, image, operation, err string, durationSeconds float64) {
	result := "success"
	if err != "" {
		result = "error"
		dockerOperationErrors.WithLabelValues(crd, operation, image, classifyDockerError(err)).Inc()
	}

	dockerOperationsTotal.WithLabelValues(crd, operation, image, result).Inc()
	dockerOperationDuration.WithLabelValues(crd, operation, image).Observe(durationSeconds)
}

// classifyDockerError categorizes Docker errors for metrics.
func classifyDockerError(err string) string {
	switch {
	case err == "":
		return "none"
	case contains(err, "not found"):
		return "not_found"
	case contains(err, "denied"):
		return "denied"
	case contains(err, "timeout"):
		return "timeout"
	case contains(err, "no space"):
		return "disk_full"
	case contains(err, "authentication"):
		return "auth"
	default:
		return "unknown"
	}
}
