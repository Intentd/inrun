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
	// reconciler is called. Never nil when Inrun calls Reconcile: a CR that
	// left the cache or is filtered out is skipped before any reconciler runs.
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

// Object returns the CR being reconciled, normalized and enriched; nil only
// for a Request built without Prepared, as in tests.
func (r Request) Object() Object {
	if r.Prepared == nil {
		return nil
	}
	return r.Prepared.Object
}

// Finalizing reports whether the CR is being deleted. A reconciler returns
// without a requeue once its cleanup is done; Inrun then removes its finalizer.
// Resources, labels and annotations returned while finalizing are ignored.
func (r Request) Finalizing() bool {
	return r.Prepared != nil && r.Prepared.Finalizing
}
