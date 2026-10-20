// pkg/types/status.go
package types

// ── Status management ──────────────────────────────────────────────────────
//
// Inrun writes status in two layers:
//
// Layer 1 — automatic standard Kubernetes conditions.
// After every reconcile, regardless of outcome, Inrun patches the CR's
// /status subresource with a standard Ready condition and observedGeneration.
// No Blueprint declaration required. Every managed CR gets this automatically,
// the same way every CR gets the managed label and finalizers.
//
// Layer 2 — declarative status fields.
// Optional. Declared in the Blueprint under operatorBox.status.fields.
// Resolved after reconcile templates complete, patched to /status.
// Field values support the same Go template expressions as onCreate templates.
//
// Example:
//
//	operatorBox:
//	  status:
//	    fields:
//	      - path: phase
//	        value: "Running"
//	      - path: observedReplicas
//	        value: "{{ .spec.replicas }}"
//	      - path: endpoint
//	        value: "{{ .metadata.name }}.{{ .metadata.namespace }}.svc.cluster.local"
//	      - path: database.host    # nested — becomes status.database.host
//	        value: "{{ .spec.host }}"
//
// Paths are relative to status — "phase" becomes status.phase.
// Dot-notation is supported for nested fields at any depth.
//
// Layer 3 — child resource status propagation adds a "children"
// context to the resolver, making child resource status fields
// accessible in path expressions.

// StatusFieldSpec declares one field to write into the CR's status.
//
// Path is relative to the status subobject. "phase" writes to status.phase.
// "database.host" writes to status.database.host.
//
// Value supports Go text/template expressions evaluated against the CR:
//   - "{{ .spec.replicas }}"    — from the CR's spec
//   - "{{ .metadata.name }}"    — CR name
//   - "Running"                 — static string (fast path, no template parsing)
//
// Type controls how the resolved value is written into the status patch.
// By default, all values are written as strings. When Type is set,
// the resolver will cast the resolved value into the requested type:
//
//	type: int       → writes an integer (e.g., 3)
//	type: float     → writes a float64 (e.g., 3.14)
//	type: bool      → writes a boolean (e.g., true)
//	type: string    → writes a string (default behavior)
//	type: auto      → attempts to infer the type from the resolved value
//
// Example:
//   - path: replicas
//     type: int
//     value: "{{ toInt .spec.replicas }}"
//
// This ensures status.replicas is written as an integer, not a string.
type StatusFieldSpec struct {
	// Path — dot-notation path relative to status.
	// "phase"            → status.phase
	// "database.host"    → status.database.host
	// "ready"            → status.ready
	Path string `yaml:"path" json:"path" validate:"required"`

	// Value — the value to write. Supports template expressions.
	// Resolved against the full CR object map at reconcile time.
	// Empty string is written as-is — declare a static empty string to clear a field.
	Value string `yaml:"value" json:"value"`

	// Type — optional explicit type for the resolved value.
	// If omitted, the value is written as a string.
	//
	// Supported values:
	//   "string", "str, "" (default)
	//   "int", "integer"
	//   "float"
	//   "bool", "boolean"
	//   "auto" — infer type from the resolved value
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// When lists conditions that must all pass for this field to be written;
	// see Condition. They are evaluated against the full CR, so .status.phase,
	// .spec.image and .children.job.status.succeeded are all available.
	When []Condition `yaml:"when,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	// Several entries for the same path, each with its own conditions, make a
	// small declarative state machine.
	Or []Condition `yaml:"or,omitempty"`

	// ClearOnFalse — when true and the when:/or: condition evaluates to false,
	// the field is explicitly written as "" rather than left untouched.
	// Use this for transient fields (e.g. crashReason) that should disappear
	// once the condition that produced them is no longer true.
	// Has no effect when no when:/or: conditions are declared.
	ClearOnFalse bool `yaml:"clearOnFalse,omitempty" json:"clearOnFalse,omitempty"`
}

// MORE NOTES ON StatusFieldSpec
//
// StatusFieldSpec extends the basic status field declaration with optional
// when: conditions. When conditions are present, the field is only written
// if all conditions pass against the current CR state.
//
// This is the primitive that makes declarative state machines possible.
// Without it, status.fields writes unconditionally after every reconcile.
// With it, different phase values can be written based on current state.
//
// Example — a three-phase state machine declared entirely in YAML:
//
//  status:
//    fields:
//      - path: phase
//        value: "Pending"
//        when:
//          - field: status.phase
//            notExists: true
//
//      - path: phase
//        value: "Running"
//        when:
//          - field: status.phase
//            equals: "Pending"
//
//      - path: phase
//        value: "Succeeded"
//        when:
//          - field: status.phase
//            equals: "Running"
//          - field: children.job.status.succeeded
//            greaterThan: "0"
//

// StatusConfig declares the declarative status behavior for a CRD.
// Declared under spec.crds[].reconciler.status in the Blueprint.
type StatusConfig struct {
	// Include is a path (relative to the blueprint file) to a YAML file whose
	// top-level value is a list of StatusFieldSpec entries under a "fields:" key.
	// Expanded at load time — included fields come first, inline fields append after.
	Include string `yaml:"include,omitempty" json:"include,omitempty"`

	// Fields — list of status fields to write after every successful reconcile.
	// Resolved in declaration order. Later fields win on path conflict.
	Fields []StatusFieldSpec `yaml:"fields,omitempty" json:"fields,omitempty"`

	// Conditions — whether to write the standard Ready condition automatically.
	// Default: true. Set to false to opt out of automatic condition management.
	// When true, Inrun writes:
	//   status.conditions[type=Ready]:
	//     status: "True" | "False"
	//     reason: ReconcileSucceeded | ReconcileError
	//     message: "" | "<error message>"
	//     lastTransitionTime: <now>
	//   status.observedGeneration: <metadata.generation>
	//
	// Setting false is rarely needed. Do it only when your CRD's status
	// schema explicitly forbids a conditions field, or when you manage
	// conditions entirely via hooks.
	Conditions *bool `yaml:"conditions,omitempty" json:"conditions,omitempty"`
}

// ConditionsEnabled reports whether automatic standard condition management is on.
// Defaults to true when Conditions is nil.
func (s *StatusConfig) ConditionsEnabled() bool {
	if s == nil || s.Conditions == nil {
		return true // default on
	}
	return *s.Conditions
}

// HasFields reports whether any declarative status fields are declared.
func (s *StatusConfig) HasFields() bool {
	return s != nil && len(s.Fields) > 0
}
