package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// reconcileMapped reconciles through a service that answers in its own shape
// (remote.response). It makes one call (request.method and request.body, or
// request.finalize while the CR is being deleted), then renders the response
// mapping with the call's result in scope as .response. The status code
// decides success: anything outside that call's expectedStatus (below 400 when
// omitted) fails the reconcile, with the usual backoff.
func (r *Reconciler) reconcileMapped(ctx context.Context, req domain.Request) (domain.Result, error) {
	resolver, ok := resolverFor(req)
	if !ok {
		return domain.Result{}, fmt.Errorf("remote reconciler: no template context for %s", req.Key)
	}

	call, ok := r.mappedCall(req.Finalizing())
	if !ok {
		// Finalizing without request.finalize: nothing to clean up outside the
		// cluster, so the CR is released.
		return domain.Result{}, nil
	}
	if met, reason := resolver.EvaluateConditions(call.when, call.or); !met {
		logger.FromContext(ctx).Info().Str("key", req.Key).Str("reason", reason).
			Msg("remote: call skipped, its when/or does not hold")
		if req.Finalizing() {
			return domain.Result{}, nil // nothing to delete: released
		}
		// No write this pass, so nothing new to map; the requeue still applies.
		return r.requeue(resolver), nil
	}

	var cs kubernetes.Interface // only an auth secretRef needs it
	if r.kube != nil {
		cs = r.kube.Clientset()
	}
	entry, err := external.Call(ctx, r.gvk.String(), "remote", resolver, call.spec, cs)
	if err != nil {
		return domain.Result{}, fmt.Errorf("remote reconciler: %w", err)
	}

	// On the request, so templates after this call, emit.status and
	// reconcile.requeue read .response too.
	resolver = resolver.WithResponse(entry)
	req.Prepared.Context = resolver
	resp := r.decl.Response
	var result domain.Result
	// A finalizing Result's writes are ignored, and a delete often has no
	// body to map, so only the requeue (which holds the CR) is evaluated.
	if !req.Finalizing() && resp.Status != nil && len(resp.Status.Fields) > 0 {
		status, err := resolver.ResolveStatusFields(resp.Status.Fields)
		if err != nil {
			return domain.Result{}, fmt.Errorf("remote reconciler: response.status: %w", err)
		}
		result.Status = status
	}
	result.Requeue = r.requeue(resolver).Requeue
	return result, nil
}

// requeue is the Result response.requeue asks for, with what is in scope.
func (r *Reconciler) requeue(resolver *template.Resolver) domain.Result {
	if after := resolver.EvaluateRequeue(r.decl.Response.Requeue); after > 0 {
		return domain.Result{Requeue: &domain.Requeue{After: after, Reason: "remote.response.requeue"}}
	}
	return domain.Result{}
}

// Hooks returns the mapped call as hooks for the generic reconciler, which
// runs it before the CRD's templates. OnDelete is nil without
// request.finalize, so the onDelete templates run instead.
func (r *Reconciler) Hooks() domain.ObjectHooks {
	call := func(ctx context.Context, req domain.Request, _ domain.Object) (domain.Result, error) {
		return r.reconcileMapped(ctx, req)
	}
	hooks := domain.ObjectHooks{OnReconcile: call}
	if rq := r.decl.Request; rq != nil && rq.Finalize != nil {
		hooks.OnDelete = call
	}
	return hooks
}

// mappedPass is the call a mapped reconcile makes this pass and the
// conditions under which it is made.
type mappedPass struct {
	spec     types.ExternalCallSpec
	when, or []types.Condition
}

// mappedCall is the one call a mapped reconcile makes, as an external call
// spec with its conditions; false when there is no call to make (finalizing
// without request.finalize).
func (r *Reconciler) mappedCall(finalizing bool) (mappedPass, bool) {
	pass := mappedPass{spec: types.ExternalCallSpec{
		Name:    "response",
		URL:     r.decl.Endpoint,
		Method:  http.MethodPost,
		Auth:    r.effectiveAuth(),
		Timeout: r.timeout.String(),
	}}
	rq := r.decl.Request
	if rq != nil {
		if rq.Method != "" {
			pass.spec.Method = rq.Method
		}
		pass.spec.Body, pass.spec.ExpectedStatus = rq.Body, rq.ExpectedStatus
		pass.when, pass.or = rq.When, rq.Or
	}
	if !finalizing {
		return pass, true
	}
	if rq == nil || rq.Finalize == nil {
		return mappedPass{}, false
	}
	f := rq.Finalize
	if f.Method != "" {
		pass.spec.Method = f.Method
	}
	if f.Endpoint != "" {
		pass.spec.URL = f.Endpoint
	}
	pass.spec.Body, pass.spec.ExpectedStatus = f.Body, f.ExpectedStatus
	pass.when, pass.or = f.When, f.Or
	return pass, true
}
