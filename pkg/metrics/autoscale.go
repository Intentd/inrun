// pkg/metrics/autoscale.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// Autoscale metrics
//
// These expose the autoscaler's behavior per operatorBox.
// Scrapable via /metrics. Visible in the Console per CRD panel.
// ─────────────────────────────────────────────────────────────────────────────

var autoscaleOverrideActiveGauge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "inrun_autoscale_override_active",
		Help: "1 when an autoscale override is currently applied to this CRD, 0 otherwise.",
	},
	[]string{"crd"},
)

var autoscaleWorkersGauge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "inrun_autoscale_workers_current",
		Help: "Current effective worker count for this CRD (baseline or override).",
	},
	[]string{"crd"},
)

var autoscaleOverridesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_autoscale_overrides_total",
		Help: "Total number of times an autoscale override was applied for this CRD.",
	},
	[]string{"crd"},
)

var autoscaleRestoresTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "inrun_autoscale_restores_total",
		Help: "Total number of times the baseline was restored for this CRD.",
	},
	[]string{"crd"},
)

// RecordAutoscaleOverride records a scale override event.
func RecordAutoscaleOverride(crd string, workers int) {
	autoscaleOverridesTotal.WithLabelValues(crd).Inc()
	autoscaleWorkersGauge.WithLabelValues(crd).Set(float64(workers))
	autoscaleOverrideActiveGauge.WithLabelValues(crd).Set(1)
}

// RecordAutoscaleRestore records a baseline restore event.
func RecordAutoscaleRestore(crd string, baselineWorkers int) {
	autoscaleRestoresTotal.WithLabelValues(crd).Inc()
	autoscaleWorkersGauge.WithLabelValues(crd).Set(float64(baselineWorkers))
	autoscaleOverrideActiveGauge.WithLabelValues(crd).Set(0)
}

// SetAutoscaleActive records an active state change.
func SetAutoscaleActive(crd string, active bool) {
	if active {
		autoscaleOverrideActiveGauge.WithLabelValues(crd).Set(1)
	} else {
		autoscaleOverrideActiveGauge.WithLabelValues(crd).Set(0)
	}
}

// InitAutoscaleBaseline records the initial worker count at startup.
// Called once per operatorBox: when the autoscaler is started.
func InitAutoscaleBaseline(crd string, workers int) {
	autoscaleWorkersGauge.WithLabelValues(crd).Set(float64(workers))
	autoscaleOverrideActiveGauge.WithLabelValues(crd).Set(0)
}
