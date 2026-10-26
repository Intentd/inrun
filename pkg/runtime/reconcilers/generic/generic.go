// pkg/reconciler/generic.go
package generic

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/tools/cache"
)

// Reconciler manages the full lifecycle of one CRD.
//
// It coordinates context enrichment, cache reads, deletion handling,
// metadata management, template execution, reconciliation priority, events,
// and logging. Resource-specific operations are implemented in separate
// resource runners.
//
// PTR must be a pointer to the concrete CR struct (for example, *Database).
// The dynamic registry path uses domain.Object, which is also supported
// because the informer cache stores the underlying concrete object.
type Reconciler[PTR domain.Object] struct {
	informer cache.SharedIndexInformer
	kube     kubeclient.Interface
	// hooks holds type-erased, domain.Object-parameterized callbacks built at
	// construction time from the user's ReconcileHooks[PTR]. Stored as
	// ObjectHooks rather than ReconcileHooks[PTR] so the reconciler remains
	// compatible with the runtime registry path (PTR = domain.Object interface).
	hooks domain.ObjectHooks

	// targetHooks holds per-target hook sets for CRDs that have distinct hook
	// binaries per serve.target entry (TargetHookFactories non-Empty().
	// Built once at construction time; read concurrently during reconcile.
	// When empty, all targets fall back to the CRD-level hooks field.
	targetHooks map[string]domain.ObjectHooks

	// callFirst runs hooks before the templates whatever runHooksFirst says:
	// a mapped remote's call, whose answer the templates read as .response.
	callFirst bool

	operatorBox *types.OperatorBoxConfig
	newObj      func() PTR
	crd         types.CRDEntry
	bp          *blueprint.Blueprint
}

// New constructs a Reconciler for the given CRD.
//
// PTR must be a pointer to the concrete CR type (e.g. *Database). When called
// from the runtime registry path in runtime_construct.go, PTR is inferred as
// domain.Object (the interface) — this is also valid because the constraint
// domain.Object is satisfied and the informer stores the correct concrete type.
//
// anyHooks, if non-nil, must implement domain.HookBinder. Every
// domain.ReconcileHooks[T] value satisfies HookBinder automatically via its
// BindToObjectHooks() method. Passing any other type panics at startup.
func New[PTR domain.Object](
	crd types.CRDEntry,
	informer cache.SharedIndexInformer,
	kube kubeclient.Interface,
	anyHooks domain.AnyReconcileHooks,
	newObj func() PTR,
	bp *blueprint.Blueprint,
) *Reconciler[PTR] {

	// Adapt the user's strongly-typed ReconcileHooks[PTR] to the type-erased
	// ObjectHooks stored on the reconciler. BindToObjectHooks wraps each hook
	// in a closure that performs obj.(PTR) at call time — safe because the
	// informer cache only ever holds objects of the type it was built for.
	var hooks domain.ObjectHooks
	if anyHooks != nil {
		binder, ok := anyHooks.(domain.HookBinder)
		if !ok {
			panic(fmt.Sprintf(
				"New[%T]: hooks value must implement domain.HookBinder "+
					"(got %T) — use domain.ReconcileHooks[*YourType]{...} or a type that "+
					"wraps one and forwards BindToObjectHooks()",
				newObj(), anyHooks,
			))
		}
		hooks = binder.BindToObjectHooks()
	}

	// Build per-target hook sets for targets that declare a distinct hook binary.
	// Targets that share the CRD-level binary only override args and use hooks above.
	factories, _ := crd.TargetHookFactories() // validated with the Blueprint
	targetHooks := make(map[string]domain.ObjectHooks, len(factories))
	for targetName, factory := range factories {
		anyH := factory()
		if binder, ok := anyH.(domain.HookBinder); ok {
			targetHooks[targetName] = binder.BindToObjectHooks()
		}
	}

	box := crd.OperatorBox

	r := &Reconciler[PTR]{
		crd:         crd,
		operatorBox: box,
		informer:    informer,
		kube:        kube,
		hooks:       hooks,
		targetHooks: targetHooks,
		newObj:      newObj,
		bp:          bp,
	}

	return r
}

var _ domain.Reconciler = (*Reconciler[domain.Object])(nil)

// WithCall makes hooks the reconciler's hooks and runs them before the
// templates: a mapped remote's call, so templates can read its answer as
// .response. Its OnDelete, when set, replaces the onDelete templates, like a
// Go hook's.
func (r *Reconciler[PTR]) WithCall(hooks domain.ObjectHooks) *Reconciler[PTR] {
	r.hooks, r.callFirst = hooks, true
	return r
}

// hooksFor returns the ObjectHooks for the given target name.
// If the target has a distinct hook binary (registered in TargetHookFactories),
// those hooks are returned. Otherwise falls back to the CRD-level hooks.
func (r *Reconciler[PTR]) hooksFor(target string) domain.ObjectHooks {
	if target != "" {
		if h, ok := r.targetHooks[target]; ok {
			return h
		}
	}
	return r.hooks
}
