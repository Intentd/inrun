package catalog

import (
	"context"
	"time"

	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EvaluateRequeue computes the requeue duration for an operatorBox
// after a successful reconcile. Returns 0 when no requeue is needed.
//
// Evaluation order:
//  1. If requeue: is absent or effectively empty, return 0.
//  2. Evaluate when/or conditions — if conditions are declared and fail, return 0.
//  3. Render the after: template against the live CR; parse as a Go duration.
//  4. Return the parsed duration (0 on parse failure — fail-open for requeue).
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

	if !types.EvaluateConditions(resolver.Data(), rq.When, rq.Or, resolver.TemplateEvaluator()) {
		return 0
	}

	if rq.After == "" {
		return 0
	}
	rendered, ok := resolver.RenderString(rq.After)
	if !ok || rendered == "" || rendered == "0s" {
		return 0
	}
	d, err := parseTimeDuration(rendered)
	if err != nil {
		return 0
	}
	return d
}
