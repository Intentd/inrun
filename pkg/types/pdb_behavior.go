package types

// PDBBehavior configures PodDisruptionBudget disruption limits.
// Set Preset for a named preset, or MinAvailable/MaxUnavailable explicitly.
// Preset and explicit fields are mutually exclusive.
type PDBBehavior struct {
	// Preset — named preset. Expands into MinAvailable or MaxUnavailable at load time.
	// Allowed: zero-downtime, rolling, relaxed.
	Preset string `yaml:"preset,omitempty" json:"preset,omitempty"`

	// MinAvailable — minimum pods that must remain available during voluntary disruptions.
	// Accepts integer string ("1") or percentage string ("100%").
	// Mutually exclusive with MaxUnavailable.
	MinAvailable string `yaml:"minAvailable,omitempty" json:"minAvailable,omitempty"`

	// MaxUnavailable — maximum pods that may be unavailable during voluntary disruptions.
	// Accepts integer string ("1") or percentage string ("25%").
	// Mutually exclusive with MinAvailable.
	MaxUnavailable string `yaml:"maxUnavailable,omitempty" json:"maxUnavailable,omitempty"`
}
