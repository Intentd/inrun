package types

// ProbeConfig configures a single Kubernetes probe (startup, liveness, or readiness).
// Supports HTTP GET and TCP socket checks, with timing driven by named presets or
// explicit field overrides.
//
// Examples:
//
//	probes:
//	  startup:
//	    type: http
//	    path: /health
//	    preset: slow-start
//	  liveness:
//	    type: http
//	    path: /health
//	    preset: standard
//	  readiness:
//	    type: http
//	    path: /ready
//	    preset: standard
//
//	probes:
//	  liveness:
//	    type: tcp
//	    preset: standard
type ProbeConfig struct {
	// Type — probe mechanism. "http" uses an HTTP GET, "tcp" opens a TCP socket.
	// When path is set and type is omitted, http is assumed.
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Path — HTTP GET path. Required when type is "http".
	Path string `yaml:"path,omitempty" json:"path,omitempty"`

	// Port — override port for the probe. Defaults to the container's declared port.
	Port int32 `yaml:"port,omitempty" json:"port,omitempty"`

	// Preset — timing preset name. Allowed values: fast, standard, patient, slow-start.
	// Defaults to "standard" when omitted.
	Preset string `yaml:"preset,omitempty" json:"preset,omitempty"`

	// Explicit timing overrides — use when presets are not granular enough.
	InitialDelaySeconds *int32 `yaml:"initialDelaySeconds,omitempty" json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       *int32 `yaml:"periodSeconds,omitempty" json:"periodSeconds,omitempty"`
	FailureThreshold    *int32 `yaml:"failureThreshold,omitempty" json:"failureThreshold,omitempty"`
	SuccessThreshold    *int32 `yaml:"successThreshold,omitempty" json:"successThreshold,omitempty"`
	TimeoutSeconds      *int32 `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
}

// ProbesConfig groups startup, liveness, and readiness probe declarations.
type ProbesConfig struct {
	Startup   *ProbeConfig `yaml:"startup,omitempty" json:"startup,omitempty"`
	Liveness  *ProbeConfig `yaml:"liveness,omitempty" json:"liveness,omitempty"`
	Readiness *ProbeConfig `yaml:"readiness,omitempty" json:"readiness,omitempty"`
}
