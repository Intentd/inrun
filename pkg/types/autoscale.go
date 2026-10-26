// pkg/types/autoscale.go
//
// Autoscale configuration for an operatorbox.
//
// The autoscaler adjusts workers, queueDepth, and resync at runtime based on
// metric conditions, time windows, and cron expressions. The CRD's declared
// values are always the baseline — overrides are temporary and automatically
// reverted when conditions are no longer met.
//
// YAML shape (inside operatorBox:):
//
//	operatorBox:
//	  autoscale:
//	    interval: 15s
//	    cooldown: 2m
//	    conditions:
//	      or:
//	        - time:
//	            after: "08:00"
//	            before: "17:00"
//	        - cron: "0 20 * * 1-5"
//	          duration: 4h
//	      when:
//	        - field: metrics.queueDepth
//	          greaterThan: "200"
//	    do:
//	      workers: 12
//	      queueDepth: 1000
//	      resync: 20s
package types

import (
	"time"

	"github.com/robfig/cron/v3"
)

// AutoscaleSpec declares the autoscale behavior for one operatorbox.
// Declared inside OperatorBoxConfig.
type AutoscaleSpec struct {
	// Interval is how often the autoscaler evaluates conditions.
	// Shorter intervals respond faster to load changes at the cost of
	// more frequent evaluation overhead (negligible in practice).
	// Default: 15s
	Interval Duration `yaml:"interval,omitempty" json:"interval,omitempty"`

	// Cooldown is the minimum time conditions must be continuously false
	// before the baseline is restored. Prevents oscillation when metrics
	// fluctuate around a threshold.
	// Default: 2m
	Cooldown Duration `yaml:"cooldown,omitempty" json:"cooldown,omitempty"`

	// Conditions declares the trigger conditions. Both blocks must pass
	// when both are declared (AND between blocks, OR within or).
	Conditions AutoscaleConditions `yaml:"conditions" json:"conditions"`

	// Do declares the override values applied when conditions are met.
	// Fields omitted in Do are not changed — only declared fields are overridden.
	Do AutoscaleAction `yaml:"do" json:"do"`

	// A preset is:
	// 	a named preset that expands into a complete autoscale configuration
	// using computed heuristics tuned for a specific behavior pattern
	Preset string `yaml:"preset,omitempty" json:"preset,omitempty"`
}

// AutoscaleConditions holds the condition blocks for autoscale evaluation.
type AutoscaleConditions struct {
	// Or — OR semantics. At least one condition in this list must be true.
	// Supports all Condition kinds: time, dayOfWeek, cron, and metric fields.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// When — AND semantics. All conditions in this list must be true.
	// Supports: metric conditions (metrics.*, cross.<crd>.metrics.*).
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`
}

// AutoscaleAction declares the override values to apply when conditions are met.
// All fields are optional pointers — only set fields are overridden.
// Unset fields retain their current value (baseline or previous override).
type AutoscaleAction struct {
	// Workers — number of concurrent reconcile goroutines.
	Workers *int `yaml:"workers,omitempty" json:"workers,omitempty"`

	// QueueDepth — maximum queue depth before backpressure.
	QueueDepth *int `yaml:"queueDepth,omitempty" json:"queueDepth,omitempty"`

	// Resync — resync interval override. How frequently all CRs are
	// re-enqueued regardless of changes.
	Resync *Duration `yaml:"resync,omitempty" json:"resync,omitempty"`
}

// AutoscaleBaseline captures the CRD's declared configuration before any
// autoscale override is applied. Always restored when conditions are false.
type AutoscaleBaseline struct {
	Workers  int
	MaxDepth int
	Resync   time.Duration
}

// AutoscaleState tracks the runtime state of the autoscaler for one operatorbox.
// Not serialized — ephemeral, reset on restart.
type AutoscaleState struct {
	// OverrideActive is true when an override is currently applied.
	OverrideActive bool

	// ConditionsFalseAt is the time when conditions last became continuously false.
	// Used to enforce the cooldown period before restoring the baseline.
	// Zero when conditions are currently true or have never been false.
	ConditionsFalseAt time.Time

	// CronWindowsOpenAt tracks when each cron-condition window opened.
	// Key is the cron expression string.
	// Without this, a cron fire that happens between two evaluation ticks would
	// be missed — the window must stay open across ticks until duration elapses.
	CronWindowsOpenAt map[string]time.Time
}

// TickCronWindow advances the cron window state machine for one expression.
// It updates windows (caller-owned state map) and returns whether the window
// is currently open.
//
// duration is how long the window stays open after a cron fire.
// interval is the evaluation tick period — used to detect fires between ticks.
// When duration is zero, interval is used as the window length.
//
// This is the general-purpose cron window tracker. Any part of Inrun that
// needs cron-gated behaviour (autoscaler, future job runner, etc.) can bring
// its own map[string]time.Time and call this on each evaluation tick.
func TickCronWindow(windows map[string]time.Time, cronExpr string, duration, interval time.Duration, now time.Time) bool {
	if duration == 0 {
		duration = interval
	}
	if duration == 0 {
		duration = 60 * time.Second
	}

	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return false
	}

	// Check whether a previously opened window is still active.
	if opened, ok := windows[cronExpr]; ok {
		if now.Before(opened.Add(duration)) {
			return true
		}
		delete(windows, cronExpr)
	}

	// Check whether the cron fired within the last interval.
	prev := schedule.Next(now.Add(-interval))
	if !prev.After(now) {
		windows[cronExpr] = prev
		return true
	}

	return false
}

// EffectiveInterval returns the autoscale evaluation interval, applying the
// default of 15s when not declared.
func (a *AutoscaleSpec) EffectiveInterval() time.Duration {
	if a.Interval.Duration == 0 {
		return 15 * time.Second
	}
	return a.Interval.Duration
}

// EffectiveCooldown returns the cooldown duration, applying the default of 2m
// when not declared.
func (a *AutoscaleSpec) EffectiveCooldown() time.Duration {
	if a.Cooldown.Duration == 0 {
		return 2 * time.Minute
	}
	return a.Cooldown.Duration
}

// HasWhenConditions returns whether when conditions are declared.
func (a *AutoscaleSpec) HasWhenConditions() bool {
	return len(a.Conditions.When) > 0
}

// HasOrConditions returns whether or: conditions are declared.
func (a *AutoscaleSpec) HasOrConditions() bool {
	return len(a.Conditions.Or) > 0
}

// HasDoWorkers returns whether do.workers is set.
func (a *AutoscaleSpec) HasDoWorkers() bool {
	return a.Do.Workers != nil
}

// HasDoQueueDepth returns whether do.queueDepth is set.
func (a *AutoscaleSpec) HasDoQueueDepth() bool {
	return a.Do.QueueDepth != nil
}

// HasDoResync returns whether do.resync is set.
func (a *AutoscaleSpec) HasDoResync() bool {
	return a.Do.Resync != nil
}

// HasIntervalDuration returns whether interval is set.
func (a *AutoscaleSpec) HasIntervalDuration() bool {
	return a.Interval.Duration != 0
}

// HasCooldownDuration returns whether cooldown is set.
func (a *AutoscaleSpec) HasCooldownDuration() bool {
	return a.Cooldown.Duration != 0
}
