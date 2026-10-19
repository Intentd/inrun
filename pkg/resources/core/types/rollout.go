package types

// RollingUpdateBehavior configures a Deployment's rolling update strategy.
// Set Preset for a named preset, or MaxSurge/MaxUnavailable explicitly.
// Preset and explicit fields are mutually exclusive.
type RollingUpdateBehavior struct {
	// Preset — named preset. Expands into MaxSurge and MaxUnavailable at load time.
	// Allowed: safe, fast, blue-green.
	Preset string `yaml:"preset,omitempty" json:"preset,omitempty"`

	// MaxSurge — maximum pods that can be scheduled above the desired replica count
	// during a rolling update. Accepts integer string ("1") or percentage string ("25%").
	MaxSurge string `yaml:"maxSurge,omitempty" json:"maxSurge,omitempty"`

	// MaxUnavailable — maximum pods that can be unavailable during a rolling update.
	// Accepts integer string ("0") or percentage string ("25%").
	MaxUnavailable string `yaml:"maxUnavailable,omitempty" json:"maxUnavailable,omitempty"`
}
