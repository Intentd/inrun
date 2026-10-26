package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	externalCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inrun_external_calls_total",
			Help: "Total number of external API calls made by the reconciler.",
		},
		[]string{"crd", "name", "url", "result"}, // result: "success", "error"
	)

	externalCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "inrun_external_call_duration_seconds",
			Help:    "Duration of external API calls.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"crd", "name", "url"},
	)

	externalCallErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inrun_external_call_errors_total",
			Help: "Total number of external API call errors, labelled by error type.",
		},
		[]string{"crd", "name", "url", "error_type"},
	)
)

// RecordExternalCall is a convenience helper for recording metrics after an external call.
func RecordExternalCall(crd, name, url string, durationSeconds float64, err string, statusCode int) {
	if err != "" {
		externalCallsTotal.WithLabelValues(crd, name, url, "error").Inc()
		errorType := "unknown"
		if statusCode > 0 {
			errorType = "http_" + string(rune(statusCode))
		} else {
			errorType = "network"
		}
		externalCallErrors.WithLabelValues(crd, name, url, errorType).Inc()
	} else {
		externalCallsTotal.WithLabelValues(crd, name, url, "success").Inc()
	}
	externalCallDuration.WithLabelValues(crd, name, url).Observe(durationSeconds)
}
