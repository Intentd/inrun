package reconcile

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// objectFromCache returns a copy of the live CR in the informer cache for the
// given key. Returns nil if the entry has no informer or the key is not found.
func objectFromCache(entry contract.RegistryEntry, key string) *unstructured.Unstructured {
	if entry.Informer == nil {
		return nil
	}
	raw, exists, err := entry.Informer.GetIndexer().GetByKey(key)
	if err != nil || !exists || raw == nil {
		return nil
	}
	obj, ok := domain.FromCache(raw)
	if !ok {
		return nil
	}
	return obj
}

// evaluatePreReconcileCheck evaluates the preReconcile.when/or gate for the
// given CR. Returns (true, reason) when gated — reconciler must not be called.
// Returns (false, "") when conditions pass.
//
// Delegates to Blueprint.EvaluatePreReconcile which holds the full resolver chain
// (profiles, notes, serve intent) — no duplication of eval logic here.
func evaluatePreReconcileCheck(
	ctx context.Context,
	d Deps,
	obj *unstructured.Unstructured,
	crdName string,
	sentinels map[string]string,
) (gated bool, reason string) {
	if d.Blueprint == nil || obj == nil {
		return false, ""
	}
	allowed, reason := d.Blueprint.EvaluatePreReconcile(ctx, crdName, obj, d.Kube.Clientset(), domain.EvaluateOptions{Sentinels: sentinels})
	return !allowed, reason
}
