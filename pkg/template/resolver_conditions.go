package template

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// EvaluateConditions evaluates when (all must hold) and or (one must hold)
// against this resolver's data. When they do not hold, reason says which
// condition failed, for logs and health. Used by preReconcile.reconcileGate
// and reconcile.when/or.
func (r *Resolver) EvaluateConditions(when, or []types.Condition) (met bool, reason string) {
	eval := r.TemplateEvaluator()
	if types.EvaluateConditions(r.Data(), when, or, eval) {
		return true, ""
	}
	for _, cond := range when {
		if !types.EvaluateConditions(r.Data(), []types.Condition{cond}, nil, eval) {
			val, _ := r.Resolve(cond.Field)
			return false, fmt.Sprintf("when: %q = %q, want %q", cond.Field, val, cond.Equals)
		}
	}
	return false, "or: no condition satisfied"
}
