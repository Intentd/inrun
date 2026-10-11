//go:build !runtime && !gateway

package gate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// AdmissionViolation is a single fired validation rule — deny or warn.
type AdmissionViolation struct {
	Field   string
	Message string
	Deny    bool
}

// AdmissionValidationResult is the outcome of running all validation rules
// for one CRD+CR pair. Passed counts rules that either did not match their
// when: guard or evaluated to no violation.
type AdmissionValidationResult struct {
	Violations []AdmissionViolation
	Passed     int
	Total      int
}

func (r AdmissionValidationResult) Denied() int {
	var n int
	for _, v := range r.Violations {
		if v.Deny {
			n++
		}
	}
	return n
}

func (r AdmissionValidationResult) Warned() int {
	var n int
	for _, v := range r.Violations {
		if !v.Deny {
			n++
		}
	}
	return n
}

// AdmissionMutationPreview describes one mutation that would be applied.
type AdmissionMutationPreview struct {
	Field   string
	Found   bool
	From    string
	To      interface{}
	MutType string // "default" or "override"
}

// AdmissionMutationResult collects all previews for one CRD+CR pair.
type AdmissionMutationResult struct {
	Previews []AdmissionMutationPreview
}

// applyMutationPreviews returns a deep copy of obj with all mutation previews
// applied. Used to simulate mutateFirst: true behaviour locally.
func applyMutationPreviews(obj map[string]interface{}, previews []AdmissionMutationPreview) map[string]interface{} {
	clone := utils.DeepCopyMap(obj)
	for _, p := range previews {
		_ = utils.SetNestedPath(clone, p.Field, p.To)
	}
	return clone
}

// EvalAdmissionValidation runs validation.rules against obj and returns the
// result. It does not print — callers format the output for their context.
func EvalAdmissionValidation(obj map[string]interface{}, crd *types.CRDEntry, resolver *template.Resolver, eval types.TemplateEvaluator) AdmissionValidationResult {
	if !crd.HasValidationRules() {
		return AdmissionValidationResult{}
	}
	validation := crd.EffectiveValidation()
	result := AdmissionValidationResult{Total: len(validation.Rules)}
	for _, rule := range validation.Rules {
		if !types.EvaluateConditions(obj, rule.When, rule.Or, eval) {
			result.Passed++
			continue
		}
		v := types.EvaluateValidationRule(obj, resolver, rule)
		if v == nil {
			result.Passed++
			continue
		}
		result.Violations = append(result.Violations, AdmissionViolation{
			Field:   v.Field,
			Message: v.Message,
			Deny:    v.Action.IsDeny(),
		})
	}
	return result
}

// EvalAdmissionMutation previews mutation.rules against obj and returns what
// would be applied. It does not print — callers format the output.
func EvalAdmissionMutation(obj map[string]interface{}, crd *types.CRDEntry, resolver *template.Resolver, eval types.TemplateEvaluator) AdmissionMutationResult {
	var result AdmissionMutationResult
	if !crd.HasMutationRules() {
		return result
	}
	for _, rule := range crd.EffectiveMutation().Rules {
		if !types.EvaluateConditions(obj, rule.When, rule.Or, eval) {
			continue
		}
		field := rule.Field
		if types.IsTemplate(field) {
			if resolved, err := resolver.Resolve(field); err == nil {
				field = resolved
			}
		}
		currentVal, found := types.ResolveScalarField(obj, field)
		desired, mutType, err := prepare.ResolveRuleValue(rule, found, currentVal, resolver)
		if err != nil || desired == nil {
			continue
		}
		if fmt.Sprintf("%v", desired) == currentVal {
			continue
		}
		result.Previews = append(result.Previews, AdmissionMutationPreview{
			Field:   field,
			Found:   found,
			From:    currentVal,
			To:      desired,
			MutType: mutType,
		})
	}
	return result
}
