// domain/generic.go
//
// Hook types for the Inrun reconcile framework.
//
// There are three layers here, each serving a different audience:
//
//  1. ReconcileHooks[T] — the user-facing API.
//     Users write: domain.ReconcileHooks[*Database]{OnReconcile: myFunc}
//     T is always a concrete pointer type (*Database, *Pipeline, etc.) because
//     Kubernetes informers store and return pointer values.
//
//  2. AnyReconcileHooks — the type-erased marker interface.
//     Allows the Catalog and ObjectRegistry to store hooks without knowing T.
//     The unexported isHooks() method prevents accidental implementation.
//
//  3. ObjectHooks / HookBinder — the internal adapter layer.
//     generic.Reconciler stores ObjectHooks, not ReconcileHooks[T], so that
//     a single reconciler type can serve both the typed user-hooks path
//     (T = *Database) and the dynamic template path (T = domain.Object)
//     that goes through the runtime registry in runtime_construct.go.
package domain

import "context"

// ReconcileHooks contains the user-provided business logic for a CRD.
// Only implement the hooks you need — all fields are optional.
//
// T must be a pointer to the concrete CR type (e.g. *Database).
// This matches how Kubernetes informers work: they store and return pointers,
// so type-asserting the informer cache entry to T succeeds only for *T, not T.
//
// Example:
//
//	func DatabaseHooks() domain.AnyReconcileHooks {
//	    return domain.ReconcileHooks[*apiv1.Database]{
//	        OnReconcile: onDatabaseReconcile,
//	        OnDelete:    onDatabaseDelete,
//	    }
//	}
//
//	func onDatabaseReconcile(ctx context.Context, req domain.Request, db *apiv1.Database) (domain.Result, error) {
//	    size := req.Args().String("size")
//	    …
//	    return domain.Result{Status: map[string]any{"size": size}}, nil
//	}
//
// A hook is a reconciler whose boilerplate is done for it: the same Request in
// and Result out as any reconciler, plus the typed object, with the Catalog's
// declarative templates applied alongside (see runHooksFirst).
type ReconcileHooks[T Object] struct {
	// OnReconcile is called for every create/update event.
	// obj is already type-asserted from the informer cache. req carries the
	// rendered reconcile.args. The Result is handled like any reconciler's:
	// status merged over emit.status, resources applied, requeue and cleanup
	// scheduled. Return an error to retry with backoff.
	OnReconcile func(ctx context.Context, req Request, obj T) (Result, error)

	// OnDelete is called when the object's DeletionTimestamp is set.
	// Use it to clean up external resources before the finalizer is removed.
	// Return a Result with a requeue to keep the finalizers and be called
	// again later (e.g. while an external teardown finishes); status,
	// resources and cleanup are ignored on delete. If nil, deletion proceeds
	// with only finalizer removal.
	OnDelete func(ctx context.Context, req Request, obj T) (Result, error)
}

// AnyReconcileHooks is the type-erased marker interface for ReconcileHooks[T].
// It allows the Catalog, ObjectRegistry, and HookFactory closures to store
// hooks without knowing the concrete T at compile time.
//
// The unexported isHooks() method prevents third-party types from accidentally
// satisfying this interface; only ReconcileHooks[T] values qualify.
type AnyReconcileHooks interface {
	isHooks() // unexported marker
}

// isHooks makes ReconcileHooks[T] satisfy AnyReconcileHooks.
func (r ReconcileHooks[T]) isHooks() {}

// ── Internal adapter layer ────────────────────────────────────────────────────
//
// generic.Reconciler stores ObjectHooks rather than ReconcileHooks[T] so it can
// hold a single concrete hooks value that works with both:
//
//   - The typed user path: T = *Database, hooks come from ReconcileHooks[*Database]
//   - The dynamic/template path: T = domain.Object (interface), hooks are nil
//
// The adapter is built once at construction time via HookBinder.BindToObjectHooks().
// Each wrapper closure performs a runtime obj.(T) assertion. This assertion is
// always safe because the informer only stores objects of the type it was
// constructed for — if the informer was built for *Database, every item in its
// cache IS a *Database even when retrieved as domain.Object.

// ObjectHooks is the type-erased counterpart to ReconcileHooks[T].
// generic.Reconciler stores this internally; users never construct it directly.
// Produce it by calling ReconcileHooks[T].BindToObjectHooks().
type ObjectHooks struct {
	OnReconcile func(ctx context.Context, req Request, obj Object) (Result, error)
	OnDelete    func(ctx context.Context, req Request, obj Object) (Result, error)
}

// HookBinder is satisfied by every ReconcileHooks[T] value.
// generic.Reconciler calls BindToObjectHooks() through this interface at
// construction time so it can adapt any typed hooks to ObjectHooks without
// knowing the concrete T.
//
// Third-party hook wrappers that embed or delegate to ReconcileHooks[T] must
// also implement HookBinder to work with generic.New.
type HookBinder interface {
	AnyReconcileHooks
	// BindToObjectHooks wraps each hook function in a closure that performs
	// a runtime obj.(T) assertion before delegating to the typed function.
	BindToObjectHooks() ObjectHooks
}

// BindToObjectHooks implements HookBinder for ReconcileHooks[T].
// The returned ObjectHooks closures assert obj to T on every call.
// This is safe because the informer guarantees every object in its cache
// is of the type it was built for.
func (h ReconcileHooks[T]) BindToObjectHooks() ObjectHooks {
	var oh ObjectHooks
	if h.OnReconcile != nil {
		fn := h.OnReconcile
		oh.OnReconcile = func(ctx context.Context, req Request, obj Object) (Result, error) {
			return fn(ctx, req, obj.(T))
		}
	}
	if h.OnDelete != nil {
		fn := h.OnDelete
		oh.OnDelete = func(ctx context.Context, req Request, obj Object) (Result, error) {
			return fn(ctx, req, obj.(T))
		}
	}
	return oh
}
