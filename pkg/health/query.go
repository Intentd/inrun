package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

// registerQueryHandler registers GET /api/v1/query on the health server mux.
// It implements enough of the Prometheus instant query HTTP API to support
// simple metric-name lookups from the default registry — sufficient for
// external: protocol: prometheus calls that target the operator's own metrics.
//
// Supported: bare metric names (go_goroutines) and label-filtered selectors
// ({__name__="go_goroutines", job="inrun"}).
// Unsupported: PromQL functions (rate, sum, etc.) — return empty vector.
func (h *HealthServer) registerQueryHandler() {
	h.mux.HandleFunc("/api/v1/query", h.queryHandler)
}

func (h *HealthServer) queryHandler(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writePromError(w, "bad_data", "query parameter is required")
		return
	}

	metricName := extractMetricName(query)
	if metricName == "" {
		// Complex PromQL — return empty vector; unsupported on this endpoint.
		writePromResult(w, "vector", []any{})
		return
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		writePromError(w, "internal", fmt.Sprintf("gathering metrics: %v", err))
		return
	}

	ts := float64(time.Now().Unix())
	var series []any
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := labelMap(m)
			val := metricValue(mf.GetType(), m)
			series = append(series, map[string]any{
				"metric": labels,
				"value":  []any{ts, val},
			})
		}
	}

	writePromResult(w, "vector", series)
}

// extractMetricName pulls the bare metric name from a PromQL expression.
// Returns empty string for expressions containing PromQL functions or operators.
func extractMetricName(query string) string {
	// Reject obvious function calls: rate(...), sum(...), etc.
	if strings.ContainsAny(query, "()+*/") {
		return ""
	}
	// Strip label selector: go_goroutines{job="foo"} → go_goroutines
	name := strings.TrimSpace(strings.SplitN(query, "{", 2)[0])
	// Metric names are [a-zA-Z_:][a-zA-Z0-9_:]*.
	for _, c := range name {
		if !isMetricNameChar(c) {
			return ""
		}
	}
	return name
}

func isMetricNameChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == ':' || c == '.'
}

func labelMap(m *dto.Metric) map[string]string {
	labels := map[string]string{}
	for _, lp := range m.GetLabel() {
		labels[lp.GetName()] = lp.GetValue()
	}
	return labels
}

func metricValue(t dto.MetricType, m *dto.Metric) string {
	var f float64
	switch t {
	case dto.MetricType_GAUGE:
		f = m.GetGauge().GetValue()
	case dto.MetricType_COUNTER:
		f = m.GetCounter().GetValue()
	case dto.MetricType_UNTYPED:
		f = m.GetUntyped().GetValue()
	default:
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func writePromResult(w http.ResponseWriter, resultType string, result []any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"status": "success",
		"data": map[string]any{
			"resultType": resultType,
			"result":     result,
		},
	})
}

func writePromError(w http.ResponseWriter, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"status":    "error",
		"errorType": errType,
		"error":     msg,
	})
}
