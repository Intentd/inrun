package types

// WorkloadAutoscale declares autoscaling behaviour for a single Deployment.
// Evaluated on every reconcile — no goroutine, no separate interval.
// The resync period controls how often conditions are checked.
//
// Example — time-based jump scaling:
//
//	autoscale:
//	  min: 2
//	  max: 10
//	  cooldown: 5m
//	  scaleUp:
//	    conditions:
//	      when:
//	        - dayOfWeek: {weekday: true}
//	        - time: {after: "09:00", before: "18:00"}
//	    target: 8
//	  scaleDown:
//	    conditions:
//	      when:
//	        - field: "{{ inBusinessHours }}"
//	          equals: "false"
//	    target: 2
type WorkloadAutoscale struct {
	// Min — replica floor. Autoscaler never scales below this value.
	// Default: the resolved value of spec.replicas at reconcile time.
	Min *int32 `yaml:"min,omitempty" json:"min,omitempty"`

	// Max — replica ceiling. Required. Autoscaler never scales above this value.
	Max int32 `yaml:"max" json:"max"`

	// Cooldown — minimum time between scale events. Prevents oscillation.
	// Accepts Go duration strings: "30s", "5m", "1h". Default: "1m".
	Cooldown Duration `yaml:"cooldown,omitempty" json:"cooldown,omitempty"`

	// ScaleUp declares the conditions and magnitude for a scale-up event.
	ScaleUp *WorkloadScaleDirection `yaml:"scaleUp,omitempty" json:"scaleUp,omitempty"`

	// ScaleDown declares the conditions and magnitude for a scale-down event.
	ScaleDown *WorkloadScaleDirection `yaml:"scaleDown,omitempty" json:"scaleDown,omitempty"`
}

// WorkloadScaleDirection declares conditions and scaling magnitude for one direction.
// Exactly one of Target, Increment (for scaleUp), or Decrement (for scaleDown) must be set.
type WorkloadScaleDirection struct {
	// Conditions — when these pass, the scale event fires (subject to cooldown and min/max).
	Conditions WorkloadScaleConditions `yaml:"conditions,omitempty" json:"conditions,omitempty"`

	// Target — jump to exactly this replica count when conditions are true.
	// Use for time-based scaling where the desired state is known.
	// Mutually exclusive with Increment/Decrement.
	Target *int32 `yaml:"target,omitempty" json:"target,omitempty"`

	// Increment — add this many replicas per scale-up event (step scaling).
	// Mutually exclusive with Target.
	Increment *int32 `yaml:"increment,omitempty" json:"increment,omitempty"`

	// Decrement — remove this many replicas per scale-down event (step scaling).
	// Mutually exclusive with Target.
	Decrement *int32 `yaml:"decrement,omitempty" json:"decrement,omitempty"`
}

// WorkloadScaleConditions holds the condition blocks for workload autoscale evaluation.
type WorkloadScaleConditions struct {
	// When — AND semantics. All conditions must be true.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or — OR semantics. At least one condition must be true.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`
}

// EffectiveCooldown returns the cooldown duration, applying the default of 1m when absent.
func (w *WorkloadAutoscale) EffectiveCooldown() Duration {
	if w.Cooldown.Duration == 0 {
		return Duration{Duration: 60 * 1e9} // 1 minute
	}
	return w.Cooldown
}
