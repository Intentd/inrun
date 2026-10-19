// pkg/resources/pdbs/types.go
package pdbs

// Spec is the fully resolved PodDisruptionBudget specification.
// All template expressions have been evaluated before this struct is populated.
// Passed directly to Create, Apply and Delete.
type Spec struct {
	// Name — PDB resource name. Required.
	Name string

	// Namespace — target namespace.
	Namespace string

	// Selector — label selector identifying the pods this PDB protects.
	Selector map[string]string

	// MinAvailable — minimum pods that must remain available.
	// Accepts integer string ("1") or percentage string ("50%").
	// Empty means not set; MaxUnavailable is used instead.
	MinAvailable string

	// MaxUnavailable — maximum pods that may be unavailable.
	// Accepts integer string ("1") or percentage string ("25%").
	// Empty means not set; MinAvailable is used instead.
	MaxUnavailable string

	// Labels applied to PDB metadata.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels map[string]string
}
