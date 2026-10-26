package prepare

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/external"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	validationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_validation_total",
			Help: "Total number of validation checks performed, labeled by result.",
		},
		[]string{"crd", "result"},
	)

	validationRejectedDetail = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "controller_validation_rejected_total",
			Help: "Validation rejections labeled by CRD, field, and rule type.",
		},
		[]string{"crd", "field", "rule"},
	)
)

// ValidationResult holds the outcome of running all validation rules for one reconcile.
type ValidationResult struct {
	Passed     bool
	Deny       bool
	Warnings   []ValidationViolation
	Violations []ValidationViolation
}

// ValidationViolation describes one failed validation rule.
type ValidationViolation struct {
	Field   string
	Rule    string
	Value   string
	Message string
	Action  types.ValidationAction
}

func (r *ValidationResult) Error() error {
	if r.Passed {
		return nil
	}
	msgs := make([]string, 0, len(r.Violations))
	for _, v := range r.Violations {
		msgs = append(msgs, fmt.Sprintf("field %q: %s (got %q)", v.Field, v.Message, v.Value))
	}
	return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
}

func (r *ValidationResult) HasWarnings() bool   { return len(r.Warnings) > 0 }
func (r *ValidationResult) HasViolations() bool { return len(r.Violations) > 0 }

func (r *ValidationResult) ViolationSummary() string {
	if r.Passed || len(r.Violations) == 0 {
		return ""
	}
	msgs := make([]string, 0, len(r.Violations))
	for _, v := range r.Violations {
		msgs = append(msgs, fmt.Sprintf("%s: %s", v.Field, v.Message))
	}
	return strings.Join(msgs, "; ")
}
func (r *ValidationResult) Denied() bool  { return r.Deny }
func (r *ValidationResult) Blocked() bool { return r.Deny }

func (r *ValidationResult) DenialError() error {
	if !r.Deny {
		return nil
	}
	return fmt.Errorf("validation denied: %s", r.DenialMessage())
}

func (r *ValidationResult) DenialMessage() string {
	if !r.Deny {
		return ""
	}
	if len(r.Violations) == 0 {
		return "validation denied"
	}
	v := r.Violations[0]
	return fmt.Sprintf("field %q: %s (got %q)", v.Field, v.Message, v.Value)
}

func (r *ValidationResult) WarningSummary() string {
	if len(r.Warnings) == 0 {
		return ""
	}
	msgs := make([]string, 0, len(r.Warnings))
	for _, w := range r.Warnings {
		msgs = append(msgs, fmt.Sprintf("field %q: %s", w.Field, w.Message))
	}
	return strings.Join(msgs, "; ")
}

// applyValidation evaluates validation rules against the live CR.
// Warn violations are logged but do not halt reconcile.
// Deny violations halt reconcile — caller patches status and returns the error.
func applyValidation(
	ctx context.Context,
	kube kubeclient.Interface,
	obj interface{ GetName() string },
	resolver *template.Resolver,
	crd types.CRDEntry,
) (*template.Resolver, *ValidationResult, error) {
	val := crd.EffectiveValidation()
	if val == nil || len(val.Rules) == 0 {
		return resolver, nil, nil
	}

	var err error
	if calls := val.ReconcileExternal(); len(calls) > 0 {
		resolver, err = external.Run(ctx, crd.GVKString(), resolver, calls, kube.Clientset())
		if err != nil {
			return resolver, nil, err
		}
	}

	result := runValidation(resolver.Data(), resolver, val, crd.GVKString())

	for _, w := range result.Warnings {
		logger.FromContext(ctx).Warn().
			Str("name", obj.GetName()).
			Str("crd", crd.GVKString()).
			Str("field", w.Field).
			Str("message", w.Message).
			Msg("reconcile validation: warn")
	}

	if result.Deny {
		return resolver, result, result.DenialError()
	}

	return resolver, result, nil
}

func runValidation(data map[string]interface{}, resolver *template.Resolver, cfg *types.ValidationConfig, crdName string) *ValidationResult {
	result := &ValidationResult{Passed: true}
	if cfg == nil || len(cfg.Rules) == 0 {
		return result
	}

	for _, rule := range cfg.Rules {
		if !rule.Fires.FiresAtReconcile() {
			continue
		}
		var eval types.TemplateEvaluator
		var tr types.TemplateResolver
		if resolver != nil {
			eval = resolver.TemplateEvaluator()
			tr = resolver
		}
		if !types.EvaluateConditions(data, rule.When, rule.Or, eval) {
			continue
		}
		ruleViolation := types.EvaluateValidationRule(data, tr, rule)
		if ruleViolation == nil {
			continue
		}

		action := types.EffectiveAction(rule.Action)
		violation := ValidationViolation{
			Field:   ruleViolation.Field,
			Rule:    ruleViolation.Rule,
			Value:   ruleViolation.Value,
			Message: ruleViolation.Message,
			Action:  action,
		}

		result.Violations = append(result.Violations, violation)
		validationRejectedDetail.WithLabelValues(crdName, rule.Field, types.RuleTypeLabel(rule)).Inc()

		switch action {
		case types.ValidationActionDeny:
			result.Deny = true
			result.Passed = false
		case types.ValidationActionWarn:
			result.Warnings = append(result.Warnings, violation)
		}
	}

	resultLabel := "passed"
	if !result.Passed {
		resultLabel = "rejected"
	} else if len(result.Warnings) > 0 {
		resultLabel = "warned"
	}
	validationTotal.WithLabelValues(crdName, resultLabel).Inc()

	if result.Deny {
		logger.Info().
			Str("crd", crdName).
			Int("violations", len(result.Violations)).
			Msg("validation: rules failed — reconciliation halted")
	}

	return result
}
