package coordinator

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// objectFromCache retrieves the live CR from the informer cache for the given
// key. Returns nil if the entry has no informer or the key is not found.
func (c *Controller) objectFromCache(entry contract.RegistryEntry, key string) *unstructured.Unstructured {
	if entry.Informer == nil {
		return nil
	}
	raw, exists, err := entry.Informer.GetIndexer().GetByKey(key)
	if err != nil || !exists || raw == nil {
		return nil
	}
	obj, ok := domain.ToUnstructured(raw)
	if !ok {
		return nil
	}
	return obj
}

// evaluatePreReconcileCheck evaluates the preReconcile.when/or gate for the
// given CR. Returns (true, reason) when gated — reconciler must not be called.
// Returns (false, "") when conditions pass.
//
// Delegates to c.cat.EvaluatePreReconcile which holds the full resolver chain
// (profiles, notes, serve intent) — no duplication of eval logic here.
func (c *Controller) evaluatePreReconcileCheck(
	ctx context.Context,
	obj *unstructured.Unstructured,
	crdName string,
	sentinels map[string]string,
) (gated bool, reason string) {
	if c.cat == nil || obj == nil {
		return false, ""
	}
	allowed, reason := c.cat.EvaluatePreReconcile(ctx, crdName, obj, c.kube.Clientset(), domain.EvaluateOptions{Sentinels: sentinels})
	return !allowed, reason
}
