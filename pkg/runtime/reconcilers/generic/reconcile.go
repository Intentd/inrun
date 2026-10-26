package generic

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply/templates"
	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// Reconcile dispatches to the correct reconcile implementation.
// Order:
//  1. Conditional provisioning (when blocks) — handled by runTemplates
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
		// Set hooks.runHooksFirst: true in the Blueprint to run the hook first;
		// a mapped remote's call always runs first.
		hooksFirst := r.callFirst || r.crd.RunHooksFirst()
		if !hooksFirst && hasTemplates {
			resolver, err = r.runTemplates(ctx, resolver, obj, box)
		}
		if err == nil {
			result, err = hooks.OnReconcile(ctx, req, obj)
		}
		if err == nil && hooksFirst && hasTemplates {
			// A hook may add to the request's resolver (a call's .response).
			resolver = req.Prepared.Context.(*template.Resolver)
			resolver, err = r.runTemplates(ctx, resolver, obj, box)
		}

	case box.EffectiveOnCreate() != nil || box.EffectiveOnReconcile() != nil:
		// Declarative templates — interpreted at runtime.
		// Requires: nothing. inrun generate registry NOT needed.
		// The returned resolver carries cross/external data for status evaluation.
		resolver, err = r.runTemplates(ctx, resolver, obj, box)

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
		if err := templates.New(r.crd, r.bp).RunOnDelete(ctx, r.kube, resolver, obj, box); err != nil {
			return domain.Result{}, fmt.Errorf("template deletion: %w", err)
		}
	}

	return domain.Result{}, nil
}
