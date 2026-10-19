package types

// CustomResourceMetadata mirrors the small subset of metav1.ObjectMeta that
// Inrun needs for templating, identity, and children tracking.
//
// Important notes:
//   - Name is required after templating. The reconciler will error if Name is empty.
//   - Namespace is optional in the struct but validation enforces it for namespaced
//     CRDs. For cluster-scoped CRDs Namespace must be empty.
//   - Labels and Annotations must conform to Kubernetes key/value rules. Use the
//     existing label/annotation validators in the codebase.
type CustomResourceMetadata struct {
	// Name is the resource name. Must be a valid Kubernetes name after templating.
	Name string `json:"name" yaml:"name"`

	// Namespace is optional in the struct but required for namespaced CRDs.
	// Validation will enforce presence when the target GVK is namespaced.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Namespaced indicates whether the target GVK is intended to be namespaced.
	// Three states are useful and intentionally supported:
	//   - nil: unspecified; reconciler should treat this as "default to namespaced"
	//          and may auto-detect the real scope via discovery at runtime.
	//   - pointer to true: explicitly namespaced; metadata.namespace is required.
	//   - pointer to false: explicitly cluster-scoped; metadata.namespace must be empty.
	//
	// Defaulting/validation contract:
	// - Validation should treat nil as "namespaced by default" for user ergonomics.
	// - At runtime the reconciler should still consult discovery/RESTMapper to
	//   confirm the actual CRD scope and register the GVK with activateMissing if
	//   the CRD is not present yet.
	// - If the user sets Namespaced=false but discovery shows the CRD is namespaced,
	//   the reconciler should surface a clear validation error.
	Namespaced *bool `json:"namespaced,omitempty" yaml:"namespaced,omitempty"`

	// Labels are used for selection, ownership, and children tracking.
	// Keys and values must conform to Kubernetes label syntax.
	Labels Labels `json:"labels,omitempty" yaml:"labels,omitempty"`

	// Annotations are free-form metadata. Keys must conform to Kubernetes
	// annotation key syntax. Values are arbitrary strings.
	Annotations Labels `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}
