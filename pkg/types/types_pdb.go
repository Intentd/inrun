// pkg/types/types_pdb.go
package types

// ── PodDisruptionBudget ───────────────────────────────────────────────────────

// PDBTemplateSource declares one PodDisruptionBudget to be managed by Inrun.
//
// Example:
//
//	onReconcile:
//	  pdb:
//	    - name: "{{ .metadata.name }}-pdb"
//	      minAvailable: "1"
//	      selector:
//	        app: "{{ .metadata.name }}"
//	      forEach:
//	        field: spec.services
//	        as: item
type PDBTemplateSource struct {
	// Name — PDB resource name. Default: "{{ .metadata.name }}-pdb"
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Namespace — target namespace. Default: CR namespace.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// Selector — label selector identifying the pods this PDB protects.
	// Keys are static; values support template expressions.
	Selector Labels `yaml:"selector,omitempty" json:"selector,omitempty"`

	// MinAvailable — minimum number of pods that must remain available.
	// Accepts integer strings ("1") or percentage strings ("50%").
	// Mutually exclusive with MaxUnavailable.
	MinAvailable string `yaml:"minAvailable,omitempty" json:"minAvailable,omitempty"`

	// MaxUnavailable — maximum number of pods that may be unavailable.
	// Accepts integer strings ("1") or percentage strings ("25%").
	// Mutually exclusive with MinAvailable.
	MaxUnavailable string `yaml:"maxUnavailable,omitempty" json:"maxUnavailable,omitempty"`

	// Labels applied to PDB metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Behavior — disruption limit configuration.
	// Set behavior.preset for a named preset, or declare minAvailable/maxUnavailable explicitly.
	// Preset and explicit fields are mutually exclusive.
	//
	//	behavior:
	//	  preset: zero-downtime
	Behavior *PDBBehavior `yaml:"behavior,omitempty" json:"behavior,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.services
	//	  as: item
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
