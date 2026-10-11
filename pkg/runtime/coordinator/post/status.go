package post

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyStatus writes Layer 1 (Ready condition) and Layer 2 (declared fields).
// It is always called — on success and failure — so Ready=False reaches the CR.
// Failures are logged as warnings and never requeue.
func applyStatus(
	ctx context.Context,
	in Input,
	obj domain.Object,
	resolver *template.Resolver,
	reconcileErr error,
	valResult *ValidationResult,
	box types.OperatorBoxConfig,
) {
	if in.CRD.SkipStatusSubresource() {
		return
	}

	patch := map[string]interface{}{}

	skipObservedGen := in.CRD.SkipObservedGeneration()
	cond := buildReadyCondition(reconcileErr, obj.GetGeneration(), skipObservedGen)
	conditions := []interface{}{cond}
	conditions = append(conditions, buildValidationCondition(valResult))
	conditions = append(conditions, buildValidationWarningCondition(valResult))
	patch["conditions"] = conditions

	if !skipObservedGen {
		patch["observedGeneration"] = obj.GetGeneration()
	}

	statusCfg := box.EffectiveStatus()

	logger.FromContext(ctx).Debug().
		Str("name", obj.GetName()).
		Bool("has_status_config", statusCfg != nil && statusCfg.HasFields()).
		Bool("reconcile_error", reconcileErr != nil).
		AnErr("reconcile_err", reconcileErr).
		Msg("status: layer2 evaluation")

	if statusCfg != nil && statusCfg.HasFields() {
		fields := statusCfg.Fields
		if reconcileErr != nil {
			var conditional []types.StatusFieldSpec
			for _, f := range fields {
				if len(f.When) > 0 || len(f.Or) > 0 {
					conditional = append(conditional, f)
				}
			}
			fields = conditional
		}
		resolved, err := resolver.ResolveStatusFields(fields)
		if err != nil {
			logger.FromContext(ctx).Warn().Err(err).
				Str("name", obj.GetName()).
				Msg("status: some fields failed to resolve")
		}
		logger.FromContext(ctx).Debug().
			Str("name", obj.GetName()).
			Interface("resolved_fields", resolved).
			Msg("status: layer2 resolved fields")
		for k, v := range resolved {
			patch[k] = v
		}
	}

	// Layer 3: the reconciler's Result.Status.
	// Applied after emit.status so the reconciler's fields win on key conflict.
	for k, v := range in.Status {
		patch[k] = v
	}

	if !statusPatchNeeded(obj, patch) {
		return
	}

	if err := in.Kube.PatchStatus(ctx, obj, patch, metav1.PatchOptions{}); err != nil {
		logger.FromContext(ctx).Warn().Err(err).
			Str("name", obj.GetName()).
			Msg("status: patch failed — continuing")
	}
}

func statusPatchNeeded(obj domain.Object, patch map[string]interface{}) bool {
	u, ok := any(obj).(*unstructured.Unstructured)
	if !ok {
		return true
	}

	if desiredGen, ok := patch["observedGeneration"].(int64); ok {
		existingGen, _, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
		if existingGen != desiredGen {
			return true
		}
	}

	existing, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	byType := make(map[string]map[string]interface{}, len(existing))
	for _, c := range existing {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := cm["type"].(string); t != "" {
			byType[t] = cm
		}
	}

	desired, _ := patch["conditions"].([]interface{})
	if len(desired) != len(byType) {
		return true
	}
	for _, d := range desired {
		dm, ok := d.(map[string]interface{})
		if !ok {
			return true
		}
		t, _ := dm["type"].(string)
		ex, found := byType[t]
		if !found {
			return true
		}
		if ex["status"] != dm["status"] || ex["reason"] != dm["reason"] || ex["message"] != dm["message"] {
			return true
		}
	}

	for k, v := range patch {
		if k == "conditions" || k == "observedGeneration" {
			continue
		}
		existing, ok, _ := unstructured.NestedFieldNoCopy(u.Object, "status", k)
		if !ok || !reflect.DeepEqual(existing, v) {
			return true
		}
	}

	return false
}

func buildReadyCondition(reconcileErr error, generation int64, skipObservedGeneration bool) map[string]interface{} {
	now := time.Now().UTC().Format(time.RFC3339)

	cond := map[string]interface{}{
		"type":               "Ready",
		"status":             "True",
		"reason":             "ReconcileSucceeded",
		"message":            "",
		"lastTransitionTime": now,
	}

	if reconcileErr != nil {
		cond["status"] = "False"
		cond["reason"] = "ReconcileError"
		msg := reconcileErr.Error()
		cond["message"] = truncateMessage(msg)
	}

	if !skipObservedGeneration {
		cond["observedGeneration"] = generation
	}

	return cond
}

func buildValidationCondition(valResult *ValidationResult) map[string]interface{} {
	now := time.Now().UTC().Format(time.RFC3339)
	if valResult != nil && valResult.Deny {
		msgs := make([]string, 0, len(valResult.Violations))
		for _, v := range valResult.Violations {
			msgs = append(msgs, fmt.Sprintf("field %q: %s (got %q)", v.Field, v.Message, v.Value))
		}
		msg := strings.Join(msgs, "; ")
		msg = truncateMessage(msg)
		return map[string]interface{}{
			"type":               "ValidationFailed",
			"status":             "True",
			"reason":             "DenyRuleViolation",
			"message":            msg,
			"lastTransitionTime": now,
		}
	}
	return map[string]interface{}{
		"type":               "ValidationFailed",
		"status":             "False",
		"reason":             "ValidationPassed",
		"message":            "",
		"lastTransitionTime": now,
	}
}

func buildValidationWarningCondition(valResult *ValidationResult) map[string]interface{} {
	now := time.Now().UTC().Format(time.RFC3339)
	if valResult != nil && len(valResult.Warnings) > 0 {
		msgs := make([]string, 0, len(valResult.Warnings))
		for _, w := range valResult.Warnings {
			msgs = append(msgs, fmt.Sprintf("field %q: %s", w.Field, w.Message))
		}
		msg := strings.Join(msgs, "; ")
		msg = truncateMessage(strings.Join(msgs, "; "))
		return map[string]interface{}{
			"type":               "ValidationWarning",
			"status":             "True",
			"reason":             "WarnRuleViolation",
			"message":            msg,
			"lastTransitionTime": now,
		}
	}
	return map[string]interface{}{
		"type":               "ValidationWarning",
		"status":             "False",
		"reason":             "NoWarnings",
		"message":            "",
		"lastTransitionTime": now,
	}
}

func truncateMessage(msg string) string {
	if len(msg) > 256 {
		msg = msg[:253] + "..."
	}
	return msg
}
