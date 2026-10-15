package catalog

import (
	"context"
	"time"

	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/template"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EvaluateRequeue computes the requeue duration for an operatorBox
// after a successful reconcile. Returns 0 when no requeue is needed.
//
// Evaluation order:
//  1. If requeue: is absent or effectively empty, return 0. For a CR being
//     deleted, a requeue without when/or conditions is a periodic refresh, not
//     a wait: return 0 so it does not hold the CR's finalizer.
//  2. Resolver.EvaluateRequeue: when/or conditions, then the after: template
//     rendered against the live CR and parsed (0 on failure: fail-open).
func (c *Catalog) EvaluateRequeue(ctx context.Context, crdName string, obj *unstructured.Unstructured, resolver *template.Resolver) time.Duration {
	if obj == nil || resolver == nil {
		return 0
	}
	entry, ok := c.CRDEntry(crdName)
	if !ok {
		return 0
	}
	target := intent.Target(obj.GetAnnotations())
	box := entry.EffectiveOperatorBox(target)
	rc := box.Reconcile
	if rc.IsRequeueEmpty() {
		return 0
	}
	rq := rc.Requeue
	if obj.GetDeletionTimestamp() != nil && len(rq.When) == 0 && len(rq.Or) == 0 {
		return 0
	}

	return resolver.EvaluateRequeue(rq)
}
