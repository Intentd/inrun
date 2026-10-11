package simulate

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// gatedReconciler wraps a domain.Reconciler and evaluates preReconcile
// conditions before delegating. When the gate fires the reconcile is a no-op —
// same behaviour as the coordinator gate in a live cluster.
type gatedReconciler struct {
	inner  domain.Reconciler
	gate   *types.PreReconcileConfig
	getObj func() *unstructured.Unstructured
	notes  types.NoteRegistry
}

func (g *gatedReconciler) Reconcile(ctx context.Context, req domain.Request) (domain.Result, error) {
	if g.gate == nil {
		return g.inner.Reconcile(ctx, req)
	}
	obj := g.getObj()
	if obj == nil {
		return g.inner.Reconcile(ctx, req)
	}
	resolver, err := template.NewResolver(ctx, obj)
	if err != nil {
		return g.inner.Reconcile(ctx, req)
	}
	eval := resolver.WithUserNotes(g.notes).TemplateEvaluator()

	// Evaluate preReconcile.enqueueGate first — mirrors informer-level drop in live path.
	if g.gate.EnqueueGate.HasConditions() {
		if !types.EvaluateConditions(resolver.Data(), g.gate.EnqueueGate.WhenConditions(), g.gate.EnqueueGate.OrConditions(), eval) {
			return domain.Result{}, nil // filtered — skip, no error
		}
	}

	// Evaluate preReconcile.when/or — mirrors coordinator gate in live path.
	if g.gate.ReconcileGate.HasConditions() {
		if !types.EvaluateConditions(resolver.Data(), g.gate.WhenConditions(), g.gate.OrConditions(), eval) {
			return domain.Result{}, nil // gated — skip, no error
		}
	}

	return g.inner.Reconcile(ctx, req)
}

// wrapWithGate returns r wrapped with a preReconcile gate check if the CRD
// declares any filter or when/or conditions; otherwise returns r unchanged.
func wrapWithGate(r domain.Reconciler, gate *types.PreReconcileConfig, notes types.NoteRegistry, getObj func() *unstructured.Unstructured) domain.Reconciler {
	if gate == nil || (!gate.ReconcileGate.HasConditions() && !gate.EnqueueGate.HasConditions()) {
		return r
	}
	return &gatedReconciler{inner: r, gate: gate, getObj: getObj, notes: notes}
}
