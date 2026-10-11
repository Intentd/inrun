// pkg/reconciler/generic.go
package generic

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/template"
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

	operatorBox *types.OperatorBoxConfig
	newObj      func() PTR
	crd         types.CRDEntry
	cat         *catalog.Catalog
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
	cat *catalog.Catalog,
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
	factories, _ := crd.TargetHookFactories() // validated with the Catalog
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
		cat:         cat,
	}

	return r
}

var _ domain.Reconciler = (*Reconciler[domain.Object])(nil)

// Reconcile dispatches to the correct reconcile implementation.
// Order:
//  1. Conditional provisioning (when blocks) — handled by runTemplateReconcile
//  2. Go hooks → Declarative templates → No-op (through reconcileImpl())
func (r *Reconciler[PTR]) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	ctx = kubeclient.WithKubeclient(ctx, r.kube)
	if err := ctx.Err(); err != nil {
		return domain.Result{}, err
	}

	ctx = logger.WithRequestID(ctx)
	ctx = logger.WithCRD(ctx, r.crd.GVKString())
	ctx = logger.WithResource(ctx, req.Key)

	if req.Prepared == nil {
		return domain.Result{}, fmt.Errorf("req.Prepared is nil for %q — all reconciles must go through coordinator", req.Key)
	}

	box := prepare.BoxFrom(req.Prepared)
	target := req.Prepared.Target
	resolver := req.Prepared.Context.(*template.Resolver)
	obj, err := domain.ToTypedWith(req.Prepared.Object, r.newObj)
	if err != nil {
		return domain.Result{}, err
	}

	hooks := r.hooksFor(target)

	// Hooks read the kube client with kubeclient.FromContext(ctx).
	ctx = kubeclient.WithKubeclient(ctx, r.kube)

	if obj.GetDeletionTimestamp() != nil {
		logger.FromContext(ctx).Info().
			Str("name", obj.GetName()).
			Msgf("deletion handler called for %s", r.crd.GVKString())
		return r.handleDeletion(ctx, req, resolver, obj, box, hooks)
	}

	result, err := r.reconcileImpl(ctx, req, resolver, obj, box, hooks)
	if err != nil {
		return domain.Result{}, err
	}

	return result, nil
}

// reconcileImpl dispatches to the correct reconcile implementation and returns
// the hook's Result (templates alone produce none).
// Priority: Go hooks → declarative templates → no-op.
func (r *Reconciler[PTR]) reconcileImpl(ctx context.Context, req domain.Request, resolver *template.Resolver, obj PTR, box types.OperatorBoxConfig, hooks domain.ObjectHooks) (domain.Result, error) {
	var (
		err    error
		result domain.Result
	)

	hasTemplates := box.EffectiveOnCreate() != nil || box.EffectiveOnReconcile() != nil
	switch {
	case hooks.OnReconcile != nil:
		// Go hooks — user-provided, full type-safe access.
		// Requires: inrun generate registry to register in HookRegistry.
		//
		// Order: by default declared templates run first (hybrid 90/10 pattern).
		// Set hooks.runHooksFirst: true in the Catalog to run the hook first.
		if !r.crd.RunHooksFirst() && hasTemplates {
			resolver, err = r.runTemplateReconcile(ctx, resolver, obj, box)
		}
		if err == nil {
			result, err = hooks.OnReconcile(ctx, req, obj)
		}
		if err == nil && r.crd.RunHooksFirst() && hasTemplates {
			resolver, err = r.runTemplateReconcile(ctx, resolver, obj, box)
		}

	case box.EffectiveOnCreate() != nil || box.EffectiveOnReconcile() != nil:
		// Declarative templates — interpreted at runtime.
		// Requires: nothing. inrun generate registry NOT needed.
		// The returned resolver carries cross/external data for status evaluation.
		resolver, err = r.runTemplateReconcile(ctx, resolver, obj, box)

	default:
		// No-op — finalizers, events, metrics still handled above.
		logger.FromContext(ctx).Info().
			Str("name", obj.GetName()).
			Msgf("reconciled %s (no-op)", r.crd.GVKString())
		// Status still patched for no-op reconcilers
	}

	if err != nil {
		logger.FromContext(ctx).Error().Err(err).
			Str("name", obj.GetName()).
			Msgf("reconciliation failed for %s", r.crd.GVKString())

		return domain.Result{}, err
	}

	logger.FromContext(ctx).Info().
		Str("name", obj.GetName()).
		Msgf("reconciled %s", r.crd.GVKString())

	return result, nil
}

// handleDeletion runs the CR's deletion logic: the OnDelete hook or onDelete
// templates. It returns without a requeue when that is done; the coordinator
// then deletes the children owner references cannot reach and removes Inrun's
// finalizer. On an error, or when OnDelete asks for a requeue, the
// finalizer stays and the CR is reconciled again; only that requeue is
// returned, since status, resources and cleanup mean nothing for an object
// being deleted.
func (r *Reconciler[PTR]) handleDeletion(ctx context.Context, req domain.Request, resolver *template.Resolver, obj PTR, box types.OperatorBoxConfig, hooks domain.ObjectHooks) (domain.Result, error) {
	switch {
	case hooks.OnDelete != nil:
		res, err := hooks.OnDelete(ctx, req, obj)
		if err != nil {
			return domain.Result{}, fmt.Errorf("deletion hook: %w", err)
		}
		if d, reason := res.EffectiveRequeue(); d > 0 {
			logger.FromContext(ctx).Info().
				Str("name", obj.GetName()).
				Str("after", d.String()).
				Str("reason", reason).
				Msg("deletion hook asked to wait; finalizers kept")
			return domain.Result{Requeue: &domain.Requeue{After: d, Reason: reason}}, nil
		}

	case box.EffectiveOnDelete() != nil:
		if err := r.runTemplateOnDelete(ctx, resolver, obj, box); err != nil {
			return domain.Result{}, fmt.Errorf("template deletion: %w", err)
		}
	}

	return domain.Result{}, nil
}
