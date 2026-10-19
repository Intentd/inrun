package types

// HPABehavior configures scale-up and scale-down behavior for a HorizontalPodAutoscaler.
// Set profile for a complete preset, or configure scaleUp/scaleDown explicitly.
// Profile and explicit fields are mutually exclusive.
type HPABehavior struct {
	// Profile — named behavior preset. Expands into scaleUp, scaleDown, and
	// targetCPUUtilizationPercentage at blueprint load time.
	// Allowed: web, api, latency-sensitive, batch, cost-optimized.
	Profile string `yaml:"profile,omitempty" json:"profile,omitempty"`

	// ScaleUp — controls how aggressively the HPA adds replicas.
	ScaleUp *HPAScalingRules `yaml:"scaleUp,omitempty" json:"scaleUp,omitempty"`

	// ScaleDown — controls how conservatively the HPA removes replicas.
	ScaleDown *HPAScalingRules `yaml:"scaleDown,omitempty" json:"scaleDown,omitempty"`
}

// HPAScalingRules configures one scaling direction (up or down).
type HPAScalingRules struct {
	// StabilizationWindowSeconds — how long the HPA observes metrics before acting.
	// Prevents flapping. ScaleDown default: 300s. ScaleUp default: 0s.
	StabilizationWindowSeconds int32 `yaml:"stabilizationWindowSeconds,omitempty" json:"stabilizationWindowSeconds,omitempty"`

	// Policies — one or more scaling policies evaluated each interval.
	// The winning policy is selected by SelectPolicy.
	Policies []HPAScalingPolicy `yaml:"policies,omitempty" json:"policies,omitempty"`

	// SelectPolicy — how to choose among policies when multiple apply.
	// "Max" (default for scaleUp), "Min" (default for scaleDown), or "Disabled".
	SelectPolicy string `yaml:"selectPolicy,omitempty" json:"selectPolicy,omitempty"`
}

// HPAScalingPolicy declares a single scaling action per period.
type HPAScalingPolicy struct {
	// Type — "Pods" (absolute count) or "Percent" (percentage of current replicas).
	Type string `yaml:"type" json:"type"`

	// Value — number of pods or percentage to scale per period.
	Value int32 `yaml:"value" json:"value"`

	// PeriodSeconds — time window for this policy (15–1800).
	PeriodSeconds int32 `yaml:"periodSeconds" json:"periodSeconds"`
}
