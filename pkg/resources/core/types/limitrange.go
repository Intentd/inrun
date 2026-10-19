package types

// LimitRangeItem sets constraints on one resource type within a namespace.
type LimitRangeItem struct {
	// Type — the resource type this item applies to.
	// One of: Container, Pod, PersistentVolumeClaim.
	Type string `yaml:"type" json:"type"`

	// Max — maximum amount of compute resources allowed.
	// Keys: cpu, memory, ephemeral-storage.
	Max map[string]string `yaml:"max,omitempty" json:"max,omitempty"`

	// Min — minimum amount of compute resources required.
	Min map[string]string `yaml:"min,omitempty" json:"min,omitempty"`

	// Default — default limit if not specified in the container spec.
	Default map[string]string `yaml:"default,omitempty" json:"default,omitempty"`

	// DefaultRequest — default resource request if not specified in the container spec.
	DefaultRequest map[string]string `yaml:"defaultRequest,omitempty" json:"defaultRequest,omitempty"`

	// MaxLimitRequestRatio — max ratio of limit to request for a resource.
	MaxLimitRequestRatio map[string]string `yaml:"maxLimitRequestRatio,omitempty" json:"maxLimitRequestRatio,omitempty"`
}
