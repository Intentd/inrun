package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// Git operation metrics
//
// These metrics track Git activity performed by the reconciler. They mirror
// the structure of external call metrics, but are specialized for Git.
//
// Labels:
//   - crd:       CRD name (GVK string)
//   - operation: "clone" | "fetch" | "checkout"
//   - repo:      repository URL
//   - result:    "success" | "error"
// ─────────────────────────────────────────────────────────────────────────────

var gitOperationsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_git_operations_total",
		Help: "Total number of Git operations performed by the reconciler.",
	},
	[]string{"crd", "operation", "repo", "result"},
)

var gitOperationDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "inrun_git_operation_duration_seconds",
		Help:    "Duration of Git operations.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"crd", "operation", "repo"},
)

var gitOperationErrors = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_git_operation_errors_total",
		Help: "Total number of Git operation errors, labelled by error type.",
	},
	[]string{"crd", "operation", "repo", "error_type"},
)

// RecordGitOperation records a Git operation result and duration.
func RecordGitOperation(crd, repo, operation, err string, durationSeconds float64) {
	result := "success"
	if err != "" {
		result = "error"
		gitOperationErrors.WithLabelValues(crd, operation, repo, classifyGitError(err)).Inc()
	}

	gitOperationsTotal.WithLabelValues(crd, operation, repo, result).Inc()
	gitOperationDuration.WithLabelValues(crd, operation, repo).Observe(durationSeconds)
}

// classifyGitError categorizes Git errors for metrics.
func classifyGitError(err string) string {
	switch {
	case err == "":
		return "none"
	case contains(err, "authentication"):
		return "auth"
	case contains(err, "not found"):
		return "not_found"
	case contains(err, "merge"):
		return "merge"
	case contains(err, "timeout"):
		return "timeout"
	default:
		return "unknown"
	}
}

func contains(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
