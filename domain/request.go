package domain

import "k8s.io/apimachinery/pkg/types"

// Request carries the identity of the object being reconciled.
// Mirrors reconcile.Request from controller-runtime so fields can be added
// without breaking the Reconciler interface.
type Request struct {
	// Key is "namespace/name" — the canonical queue identifier.
	Key string
	// NamespacedName is parsed from Key for convenience.
	NamespacedName types.NamespacedName
	// Prepared holds the runtime-enriched context built by Coordinator before the
	// reconciler is called.
	Prepared *PreparedRequest
}

// String returns the Key — satisfies fmt.Stringer and matches
// the ctrl.Request.String() behaviour that migrated code may reference.
func (r Request) String() string { return r.Key }

// Namespace returns the namespace component of the key.
func (r Request) Namespace() string { return r.NamespacedName.Namespace }

// Name returns the name component of the key.
func (r Request) Name() string { return r.NamespacedName.Name }

// Args returns the prepared args already resolved before reconcile is called
func (r Request) Args() Args {
	if r.Prepared == nil {
		return Args{}
	}
	return r.Prepared.Args
}
