package types

// NormalizeConfig declares template-driven spec normalization.
// This phase runs BEFORE mutation, validation, and reconciliation.
//
// Purpose:
//   - Accept multiple user-facing shapes (string vs map, list vs scalar, etc.)
//   - Collapse them into a single canonical spec used internally.
//   - Avoid drift where the CR stores one shape but children require another.
//   - Provide a declarative alternative to conversion webhooks.
//
// Behavior:
//   - normalize.spec is a map of field → template string.
//   - Each template is evaluated against the RAW CR object.
//   - The rendered values overwrite the corresponding fields in .spec.
//   - Only declared fields are overwritten; others remain untouched.
//   - The normalized object is passed to NewResolver() and all downstream phases.
//   - The stored CR in etcd is NOT modified.
//
// Nested paths are supported:
//
//	normalize:
//	  spec:
//	    resources.requests.cpu: "{{ default .spec.resources.requests.cpu \"100m\" }}"
//	    containers.0.image: "{{ .spec.image }}:{{ .spec.tag }}"
//
// NormalizeConfig declares field normalizations that run before mutation,
// validation, and template rendering.
//
// Keys are dot-notation paths into spec (e.g. "schedule", "resources.limits.cpu").
// Values are template expressions evaluated against the raw CR.
// Results are written back into the in-memory spec copy.
type NormalizeConfig struct {
	// Spec contains field-level normalization templates.
	// Example:
	//
	//   normalize:
	//     spec:
	//       schedule: >
	//         {{ if typeMap .spec.schedule }}
	//           {{ cronFromMap .spec.schedule }}
	//         {{ else }}
	//           {{ cronNormalize .spec.schedule }}
	//         {{ end }}
	//
	// Spec maps a dot-notation field path to a template expression.
	// The template sees the raw CR (before any normalization of other fields).
	// Results are coerced to the appropriate Go type via YAML parsing:
	//   "3"     → int
	//   "true"  → bool
	//   "*/5 * * * *" → string
	// Empty result ("") sets the field to empty string — not nil.
	Spec map[string]string `yaml:"spec,omitempty" json:"spec,omitempty"`

	// Audit enables per-field change tracking during normalize.
	// When true, Inrun records which spec fields were transformed and what
	// value they held before normalization. The changes are available in status
	// field templates under ._normalizeChanges as a list of {field, from, to}.
	//
	// Example:
	//
	//   normalize:
	//     audit: true
	//     spec:
	//       environment: '{{ default "production" .spec.environment | toLower }}'
	//
	//   status:
	//     fields:
	//       - path: normalizeChanges
	//         value: "{{ toJson ._normalizeChanges }}"
	Audit bool `yaml:"audit,omitempty" json:"audit,omitempty"`

	// Types declares the expected output type for specific spec fields.
	// When a field is listed here, its rendered value is cast to the declared
	// type instead of being inferred via YAML parsing. This is more precise:
	//   - An empty string ("") for a typed field writes nil (omits the field)
	//     rather than an empty string, which would fail integer/boolean schema validation.
	//   - "0" declared as bool → false, not int64(0).
	//   - "false" declared as bool → false even when YAML would parse it correctly.
	//
	// Accepted type values: "int", "integer", "bool", "boolean", "float", "number", "string".
	//
	// Example:
	//
	//   normalize:
	//     spec:
	//       replicas: "{{ default 1 .spec.replicas }}"
	//       suspend:  "{{ default false .spec.suspend }}"
	//     types:
	//       replicas: int
	//       suspend:  bool
	Types map[string]string `yaml:"types,omitempty" json:"types,omitempty"`
}

// NormalizeChange records a single field transformation during normalize.
// Available in status templates as ._normalizeChanges when audit: true.
type NormalizeChange struct {
	Field string      `json:"field"`
	From  interface{} `json:"from,omitempty"`
	To    interface{} `json:"to,omitempty"`
}
