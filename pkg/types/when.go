// pkg/types/when.go
//
// EvaluateConditions — when: (all must pass) and or: (one must pass), run by
// github.com/intentd/tools/condition.
//
//	# Combined: (spec.enabled=true) AND (phase=Failed OR phase=Succeeded)
//	when:
//	  - field: spec.enabled
//	    equals: "true"
//	or:
//	  - field: status.phase
//	    equals: "Failed"
//	  - field: status.phase
//	    equals: "Succeeded"
package types

import (
	"time"

	"github.com/intentd/tools/condition"
	"github.com/intentd/tools/structured"
)

// TemplateEvaluator evaluates a Go template expression against the current
// reconcile data map, including all KEL functions and enriched children.
// Returns (result, true) on success; ("", false) on any error.
// Implemented by the resolver package and injected at call sites.
// nil disables template evaluation
type TemplateEvaluator = condition.TemplateEvaluator

// IsTemplate reports whether s contains a Go template expression.
func IsTemplate(s string) bool { return structured.IsTemplate(s) }

// EvaluateConditions evaluates when: (allOf, AND) and or: (OR) conditions.
// data is resolver.Data() — full CR map including children, external, cross.
// eval is optional — pass nil to disable template evaluation
//
// Both blocks must pass when both are declared.
// Empty blocks always pass.
func EvaluateConditions(data map[string]interface{}, allOf Conditions, or Conditions, eval TemplateEvaluator) bool {
	return condition.Evaluate(data, allOf.Unwrap(), or.Unwrap(), eval)
}

// EvaluateOneCond evaluates a single Condition against a data map.
func EvaluateOneCond(data map[string]interface{}, cond Condition, eval TemplateEvaluator) bool {
	return condition.EvaluateOne(data, cond.Condition, eval)
}

// NavigateRawPath walks a dot-notation path through a nested map.
// Returns nil when any segment is missing — the notExists case.
func NavigateRawPath(m map[string]interface{}, path string) interface{} {
	return condition.NavigateRawPath(m, path)
}

// NavigateDotPath walks a dot-notation path through a nested map.
// Returns "" when any segment is missing — the notExists case.
func NavigateDotPath(m map[string]interface{}, path string) string {
	return condition.NavigateDotPath(m, path)
}

// ResolveConditionOp resolves the effective operator and comparison value
// from a Condition, respecting shorthand fields.
func ResolveConditionOp(c Condition) (ConditionOperator, string) {
	return condition.ResolveOp(c.Condition)
}

// EvalDayOfWeekAt reports whether now's day matches d.
func EvalDayOfWeekAt(d *DayOfWeekCondition, now time.Time) bool {
	return condition.DayOfWeekAt(d, now)
}
