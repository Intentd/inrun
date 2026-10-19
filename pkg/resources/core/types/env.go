package types

// ResourceRequirements mirrors Kubernetes resource requests and limits.
// Values are static Kubernetes quantity strings — template expressions
// are not supported here.
// e.g. requests: {cpu: "100m", memory: "128Mi"}
//
// Preset is mutually exclusive with Requests and Limits. When Preset is set,
// it expands into a complete ResourceRequirements at reconcile time.
type ResourceRequirements struct {
	Preset   string            `yaml:"preset,omitempty" json:"preset,omitempty" validate:"omitempty"`
	Requests map[string]string `yaml:"requests,omitempty" json:"requests,omitempty" validate:"omitempty"`
	Limits   map[string]string `yaml:"limits,omitempty" json:"limits,omitempty" validate:"omitempty"`
}

// EnvVar is a single container environment variable in Kubernetes-native format.
type EnvVar struct {
	Name      string     `yaml:"name" json:"name"`
	Value     string     `yaml:"value,omitempty" json:"value,omitempty"`
	ValueFrom *ValueFrom `yaml:"valueFrom,omitempty" json:"valueFrom,omitempty"`
}

// ValueFrom holds the source of an environment variable value.
// At most one field should be set.
type ValueFrom struct {
	SecretKeyRef    *SecretKeyRef    `yaml:"secretKeyRef,omitempty" json:"secretKeyRef,omitempty"`
	ConfigMapKeyRef *ConfigMapKeyRef `yaml:"configMapKeyRef,omitempty" json:"configMapKeyRef,omitempty"`
}

// SecretKeyRef selects a key from a Kubernetes Secret.
// Name and Key are required. Optional mirrors corev1.SecretKeySelector.Optional.
type SecretKeyRef struct {
	Name     string `yaml:"name" json:"name"`
	Key      string `yaml:"key" json:"key"`
	Optional *bool  `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// ConfigMapKeyRef selects a key from a Kubernetes ConfigMap.
// Name and Key are required. Optional mirrors corev1.ConfigMapKeySelector.Optional.
type ConfigMapKeyRef struct {
	Name     string `yaml:"name" json:"name"`
	Key      string `yaml:"key" json:"key"`
	Optional *bool  `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// EnvFromRef is one secretRef/configMapRef entry under envFrom.
//
// Name, Prefix, and Optional map directly onto Kubernetes' own EnvFromSource /
// SecretEnvSource / ConfigMapEnvSource fields. Keys and Suffix are Inrun
// extensions with no Kubernetes equivalent — Kubernetes' envFrom is a blanket
// import with no per-key rename mechanism, so a ref that sets Keys is expanded
// into individual env entries at build time instead of a native envFrom source.
// Suffix without Keys is a validation error — see pkg/blueprint/validate_envfrom.go.
type EnvFromRef struct {
	Name     string   `yaml:"name" json:"name" validate:"required"`
	Prefix   string   `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Suffix   string   `yaml:"suffix,omitempty" json:"suffix,omitempty"`
	Optional *bool    `yaml:"optional,omitempty" json:"optional,omitempty"`
	Keys     []string `yaml:"keys,omitempty" json:"keys,omitempty"`
}

// EnvFrom groups environment sources by type.
type EnvFrom struct {
	SecretRef    []EnvFromRef `yaml:"secretRef,omitempty" json:"secretRef,omitempty"`
	ConfigMapRef []EnvFromRef `yaml:"configMapRef,omitempty" json:"configMapRef,omitempty"`
}
