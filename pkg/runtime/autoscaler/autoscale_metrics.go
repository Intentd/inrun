// pkg/autoscaler/autoscale_metrics.go
//
// AutoMetrics — live operatorbox metrics read directly from the runtime.
//
// The autoscaler evaluates conditions against these metrics on every tick.
// All reads are atomic — no locks, no API calls, no informer lookups.
//
// Metric fields are exposed under the "metrics.*" namespace in autoscale
// conditions and resolve through AutoMetrics.Get(field).
package autoscaler

import (
	"math"
	"strconv"
	"sync/atomic"
	"time"
)

type runtimeMetrics string

const (
	metricsWorkersBusyPercent   runtimeMetrics = "metrics.workersBusyPercent"
	metricsWorkersIdlePercent   runtimeMetrics = "metrics.workersIdlePercent"
	metricsQueueDepth           runtimeMetrics = "metrics.queueDepth"
	metricsReconcileDurationP95 runtimeMetrics = "metrics.reconcileDurationP95Ms"
	metricsErrorRatePercent     runtimeMetrics = "metrics.errorRatePercent"
)

// AutoMetrics holds live operatorbox runtime metrics for autoscale evaluation.
// All counters are atomic int64 values — safe for concurrent read/write from
// worker goroutines and the autoscaler loop.
type AutoMetrics struct {
	// reconcileTotal is the total number of reconciles since startup.
	reconcileTotal atomic.Int64

	// errorRatePercent is the percentage error in reconciles since startup
	errorRatePercent atomic.Int64

	// reconcileErrors is the total number of failed reconciles since startup.
	reconcileErrors atomic.Int64

	// queueDepth is the current number of items in the workqueue.
	queueDepth atomic.Int64

	// workerSem is the worker semaphore — provides in-flight and capacity.
	workerSem *ResizableSemaphore

	// p95DurationNs tracks a rolling P95 of reconcile durations in nanoseconds.
	// Updated on each reconcile completion.
	p95 *rollingP95
}

// NewAutoMetrics returns an initialised AutoMetrics for the given semaphore.
func NewAutoMetrics(sem *ResizableSemaphore) *AutoMetrics {
	return &AutoMetrics{
		workerSem: sem,
		p95:       newRollingP95(256), // last 256 reconcile durations
	}
}

// RecordReconcile records one completed reconcile with its duration and outcome.
// Called at the end of every reconcile, success or failure.
func (m *AutoMetrics) RecordReconcile(duration time.Duration, failed bool) {
	m.reconcileTotal.Add(1)
	if failed {
		m.reconcileErrors.Add(1)
	}
	m.p95.record(int64(duration))
}

// SetQueueDepth updates the current queue depth. Called by the workqueue wrapper
// on every enqueue and dequeue.
func (m *AutoMetrics) SetQueueDepth(depth int64) {
	m.queueDepth.Store(depth)
}

// Get returns the string representation of a metric field for condition evaluation.
// Field names follow the metrics.* convention:
//
//	metrics.workersBusyPercent   → "73.5"
//	metrics.workersIdlePercent   → "26.5"
//	metrics.queueDepth           → "342"
//	metrics.reconcileDurationP95Ms → "47"
//	metrics.errorRatePercent     → "0.2"
//
// Returns "" for unknown fields — evaluates as notExists in condition checks.
func (m *AutoMetrics) Get(field string) string {
	switch field {
	case string(metricsWorkersBusyPercent):
		return formatFloat(m.workerSem.BusyPercent())

	case string(metricsWorkersIdlePercent):
		return formatFloat(m.workerSem.IdlePercent())

	case string(metricsQueueDepth):
		return strconv.FormatInt(m.queueDepth.Load(), 10)

	case string(metricsReconcileDurationP95):
		return formatFloat(m.p95.p95Milliseconds())

	case string(metricsErrorRatePercent):
		return formatFloat(m.ErrorRatePercent())

	default:
		return ""
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(math.Round(f*10)/10, 'f', 1, 64)
}

// ──────────────────────────────────────────────────────────────────────────────
// Rolling P95 — approximate P95 over the last N observations.
// Uses a circular buffer of samples. No heap allocation after construction.
// ──────────────────────────────────────────────────────────────────────────────

type rollingP95 struct {
	buf  []int64
	pos  int
	full bool
}

func newRollingP95(size int) *rollingP95 {
	return &rollingP95{buf: make([]int64, size)}
}

func (r *rollingP95) record(v int64) {
	r.buf[r.pos] = v
	r.pos = (r.pos + 1) % len(r.buf)
	if r.pos == 0 {
		r.full = true
	}
}

func (r *rollingP95) p95() int64 {
	n := len(r.buf)
	if !r.full {
		n = r.pos
	}
	if n == 0 {
		return 0
	}
	// Copy and sort
	tmp := make([]int64, n)
	copy(tmp, r.buf[:n])
	sortInt64(tmp)
	idx := int(math.Ceil(float64(n)*0.95)) - 1
	if idx < 0 {
		idx = 0
	}
	return tmp[idx]
}

func (r *rollingP95) p95Milliseconds() float64 {
	ns := r.p95()
	if ns == 0 {
		return 0
	}
	return float64(ns) / float64(time.Millisecond)
}

// sortInt64 is a simple insertion sort — adequate for n ≤ 256.
func sortInt64(a []int64) {
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

// IsMetricField returns true when the field path refers to an autoscale metric.
// Used by the condition resolver to route metric lookups to AutoMetrics.Get.
func IsMetricField(field string) bool {
	return len(field) > 8 && field[:8] == "metrics."
}

func (m *AutoMetrics) AsMap() map[string]interface{} {
	return map[string]interface{}{
		"queueDepth":             m.queueDepth.Load(),
		"workers":                m.workerSem.Capacity(),
		"workersBusyPercent":     m.workerSem.BusyPercent(),
		"workersIdlePercent":     m.workerSem.IdlePercent(),
		"reconcileDurationP95Ms": m.p95.p95Milliseconds(),
		"errorRatePercent":       m.ErrorRatePercent(),
	}
}

// ErrorRatePercent returns the curent error rate i percentage
func (m *AutoMetrics) ErrorRatePercent() float64 {
	total := m.reconcileTotal.Load()
	if total == 0 {
		return 0
	}
	rate := float64(m.reconcileErrors.Load()) / float64(total) * 100
	m.errorRatePercent.Store(int64(rate))
	return rate
}
