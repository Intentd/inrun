package types

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DefaultFieldManager is the server-side apply field manager when Options
// names none.
const DefaultFieldManager = "kuberesources"

// Options says how Apply writes with server-side apply.
type Options struct {
	// FieldManager owns the fields Apply sets. Empty means DefaultFieldManager.
	FieldManager string

	// Force takes ownership of fields another manager set, instead of
	// returning a conflict.
	Force bool
}

// PatchOptions returns the options for a server-side apply patch.
func (o Options) PatchOptions() metav1.PatchOptions {
	fm := o.FieldManager
	if fm == "" {
		fm = DefaultFieldManager
	}
	return metav1.PatchOptions{FieldManager: fm, Force: &o.Force}
}
