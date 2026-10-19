// pkg/autoscaler/autoscale_worker_info.go
//
// WorkerInfo — the complete picture of an operatorbox's worker state.
// Returned by the CRD handler endpoint and shown in the Console.
//
// The Console shows:
//
//	Configured: 4      ← CRD declared workers (always the baseline)
//	Effective:  12     ← current semaphore capacity (may be overridden)
//	InFlight:   7      ← goroutines currently reconciling
//	Idle:       5      ← effective - inFlight
//	Max:        12     ← highest workers ever from do: block (pre-allocated)
//	Autoscaler: ✓ enabled, override active
//
// This replaces the misleading "N pending workers" display that showed all
// pre-allocated goroutines as pending regardless of autoscale state.
package autoscaler

// WorkerInfo is the serializable worker state for the CRD handler API.
// Keyed as "workers" in the CRD detail JSON response.
type WorkerInfo struct {
	// Configured is the baseline workers declared in the CRD entry.
	// This is what the operatorbox returns to after an autoscale override ends.
	Configured int `json:"configured"`

	// Effective is the current semaphore capacity — the real concurrency limit.
	// Equals Configured when no autoscale override is active.
	// Equals the override value when autoscale is active.
	Effective int `json:"effective"`

	// InFlight is the number of goroutines currently inside a reconcile call.
	// InFlight <= Effective always.
	InFlight int `json:"inFlight"`

	// Idle is how many effective worker slots are available right now.
	// Idle = Effective - InFlight.
	Idle int `json:"idle"`

	// Max is the maximum workers the autoscaler can scale to (from do.workers).
	// Zero when autoscaler is not declared.
	// This is pre-allocated — the goroutine pool is sized to Max at startup.
	Max int `json:"max,omitempty"`

	// AutoscalerEnabled is true when an autoscale: block is declared.
	AutoscalerEnabled bool `json:"autoscalerEnabled"`

	// OverrideActive is true when an autoscale override is currently applied.
	OverrideActive bool `json:"overrideActive,omitempty"`

	// OverrideWorkers is the current override value when OverrideActive is true.
	OverrideWorkers int `json:"overrideWorkers,omitempty"`

	// QueueDepth is the current number of items in the workqueue.
	QueueDepth int64 `json:"queueDepth"`

	// QueueDepthConfigured is the baseline queueDepth from the CRD entry.
	QueueDepthConfigured int `json:"queueDepthConfigured"`

	// QueueDepthEffective is the current queue depth limit (baseline or override).
	QueueDepthEffective int `json:"queueDepthEffective"`

	// BusyPercent is InFlight/Effective as a percentage. Useful for UI gauges.
	BusyPercent float64 `json:"busyPercent"`

	// ResyncEffective is the current resync interval (baseline or override).
	ResyncEffective string `json:"resyncEffective"`

	// ResyncConfigured is the baseline resync from the CRD entry.
	ResyncConfigured string `json:"resyncConfigured"`
}

// BuildWorkerInfo constructs the WorkerInfo from the live operatorbox state.
// Called by the CRD handler on every /blueprint/{crd} request — reads are O(1).
func BuildWorkerInfo(
	sem *ResizableSemaphore,
	metrics *AutoMetrics,
	configured int,
	configuredQueueDepth int,
	configuredResync string,
	maxWorkers int,
	autoscalerEnabled bool,
	autoscalerState *autoscalerStateSnapshot,
) WorkerInfo {
	effective := sem.Capacity()
	inFlight := sem.InFlight()
	idle := effective - inFlight
	if idle < 0 {
		idle = 0
	}

	queueDepth := metrics.queueDepth.Load()

	info := WorkerInfo{
		Configured:           configured,
		Effective:            effective,
		InFlight:             inFlight,
		Idle:                 idle,
		Max:                  maxWorkers,
		AutoscalerEnabled:    autoscalerEnabled,
		QueueDepth:           queueDepth,
		ResyncConfigured:     configuredResync,
		ResyncEffective:      configuredResync, // updated below if override active
		QueueDepthConfigured: configuredQueueDepth,
		QueueDepthEffective:  configuredQueueDepth, // updated below if override active
		BusyPercent:          sem.BusyPercent(),
	}

	if autoscalerState != nil && autoscalerState.OverrideActive {
		info.OverrideActive = true
		info.OverrideWorkers = autoscalerState.EffectiveWorkers
		info.QueueDepthEffective = autoscalerState.EffectiveQueueDepth
		info.ResyncEffective = autoscalerState.EffectiveResync
	}

	return info
}

// autoscalerStateSnapshot is a point-in-time snapshot of autoscale state
// for the CRD handler — avoids locking the Autoscaler struct on reads.
type autoscalerStateSnapshot struct {
	OverrideActive      bool
	EffectiveWorkers    int
	EffectiveQueueDepth int
	EffectiveResync     string
}

// Snapshot returns a point-in-time snapshot of the Autoscaler's state.
// Called by the CRD handler without holding any lock — reads atomic values.
func (a *Autoscaler) Snapshot() *autoscalerStateSnapshot {
	if a == nil {
		return nil
	}
	workers := a.baseline.Workers
	qdepth := a.baseline.MaxDepth
	resync := a.baseline.Resync
	overrideActive := a.state.OverrideActive

	if overrideActive {
		if a.spec.Do.Workers != nil {
			workers = *a.spec.Do.Workers
		}
		if a.spec.Do.QueueDepth != nil {
			qdepth = *a.spec.Do.QueueDepth
		}
		if a.spec.Do.Resync != nil {
			resync = a.spec.Do.Resync.Duration
		}
	}

	return &autoscalerStateSnapshot{
		OverrideActive:      overrideActive,
		EffectiveWorkers:    workers,
		EffectiveQueueDepth: qdepth,
		EffectiveResync:     resync.String(),
	}
}
