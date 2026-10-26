package template

import (
	"time"

	"github.com/intentd/kel/core"

	"github.com/intentd/inrun/pkg/types"
)

// EvaluateRequeue returns the delay a requeue block asks for against this
// resolver's data: 0 when it is empty, its when/or conditions fail, or after
// renders to nothing or an unparseable duration (fail-open: no requeue).
// Used by reconcile.requeue and by a remote reconciler's response mapping.
func (r *Resolver) EvaluateRequeue(rq *types.RequeueConfig) time.Duration {
	if r == nil || rq == nil || rq.After == "" {
		return 0
	}
	if !types.EvaluateConditions(r.Data(), rq.When, rq.Or, r.TemplateEvaluator()) {
		return 0
	}
	rendered, ok := r.RenderString(rq.After)
	if !ok || rendered == "" || rendered == "0s" {
		return 0
	}
	d, err := core.ParseTimeDuration(rendered)
	if err != nil {
		return 0
	}
	return d
}
