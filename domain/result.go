package domain

import "time"

// Result is returned by Reconcile: what the reconciler wants the runtime to do
// next. The runtime performs every write; the reconciler only declares.
// Zero value means: no special scheduling — the runtime's resync and queue handle it.
type Result struct {
	// Requeue schedules an exact re-enqueue of this object. Takes precedence
	// over RequeueAfter when both are set. Honored only on successful reconcile;
	// the error path uses retry backoff.
	Requeue *Requeue

	// RequeueAfter is shorthand for Requeue{After: d}. Ignored if zero.
	RequeueAfter time.Duration

	// Cleanup asks the runtime to delete this object. nil means no cleanup;
	// &Cleanup{} means now. The first request starts the clock: later requests
	// do not move the due time. Returning nil withdraws a pending request.
	// Independent of Requeue: whichever is due first acts.
	Cleanup *Cleanup

	// Status is an optional set of fields to merge into the CR's status after
	// reconcile. Keys are applied after catalog emit.status fields — these win
	// on conflict. Ignored when nil or empty.
	Status map[string]interface{}

	// Resources are Kubernetes objects to apply with server-side apply, in full
	// form (apiVersion, kind, metadata, …) or intent form ({"type", "fields"}).
	// Each must be declared in the CRD's managedResources. Objects in the CR's
	// namespace get the CR as owner.
	//
	// 	Limitation:
	// 	  Not yet pruned: dropping a resource from the list does not delete it.
	Resources []map[string]interface{}

	// Labels and Annotations are set on the CR itself with server-side apply.
	// They are reasserted on every reconcile; a key dropped from a later Result
	// is removed, and keys set by anyone else are never touched. Keys under
	// inrun.dev/ are reserved for the runtime and rejected.
	Labels      map[string]string
	Annotations map[string]string
}

// Requeue asks the runtime to reconcile the object again after a delay.
type Requeue struct {
	// After is the delay before the next reconcile.
	After time.Duration
	// Reason is shown in logs; optional.
	Reason string
}

// Cleanup asks the runtime to delete the object after a delay.
type Cleanup struct {
	// After is the grace period before deletion; zero means now.
	After time.Duration
	// Reason is recorded in the deletion event and logs; optional.
	Reason string
}

// EffectiveRequeue returns the requested requeue delay and its reason.
// Requeue wins over RequeueAfter; a zero delay means no requeue was requested.
func (r Result) EffectiveRequeue() (time.Duration, string) {
	if r.Requeue != nil {
		return r.Requeue.After, r.Requeue.Reason
	}
	return r.RequeueAfter, ""
}
