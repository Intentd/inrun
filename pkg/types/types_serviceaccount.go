// pkg/types/types_serviceaccount.go
package types

// ── ServiceAccount ────────────────────────────────────────────────────────────

// ServiceAccountTemplateSource declares one ServiceAccount to be managed by Inrun.
//
// Example:
//
//	onCreate:
//	  serviceAccounts:
//	    - name: "{{ .metadata.name }}-sa"
//	      namespace: "{{ .metadata.namespace }}"
//	      labels:
//	        app: "{{ .metadata.name }}"
type ServiceAccountTemplateSource struct {
	// Name — ServiceAccount name.
	// Default when omitted: "{{ .metadata.name }}-sa"
	Name string `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`

	// Namespace — target namespace.
	// Default when omitted: "{{ .metadata.namespace }}"
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" validate:"omitempty"`

	// Labels — applied to ServiceAccount metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty" validate:"omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty" validate:"omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// ── Namespace ────────────────────────────────────────────────────────────

// NamespaceTemplateSource declares one Namespace to be managed by Inrun.
//
// Example:
//
//	onCreate:
//	  namespaces:
//	    - name: "{{ .metadata.name }}-ns"
//	      labels:
//	        app: "{{ .metadata.name }}"
type NamespaceTemplateSource struct {
	// Name — Namespace name.
	// Default when omitted: "{{ .metadata.name }}-ns"
	Name string `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`

	// Finalizers — additional Kubernetes finalizers to add to the Namespace,
	// beyond the standard "kubernetes" finalizer. The Namespace stays in
	// Terminating state until every finalizer is removed by its owning
	// controller, so only add one if something must run cleanup before this
	// Namespace can actually be deleted.
	//
	//	finalizers:
	//	  - "{{ .metadata.name }}/cleanup"
	Finalizers []string `yaml:"finalizers,omitempty" json:"finalizers,omitempty" validate:"omitempty"`

	// Labels — applied to Namespace metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty" validate:"omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty" validate:"omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
