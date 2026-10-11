package prepare

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/inrundev/inrun/pkg/utils"
	"strconv"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/external"
	"github.com/inrundev/inrun/pkg/kubeclient"
	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/metrics"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// MutationResult holds the outcome of applying mutation rules.
type MutationResult struct {
	Applied int
	Changes []MutationChange
}

// MutationChange describes one field mutation that was applied.
type MutationChange struct {
	Field    string
	OldValue string
	NewValue interface{}
	Type     string // "default" or "override"
}

// applyMutation applies mutation rules to the CR and patches it when values change.
// Returns the enriched resolver (external call results available to caller).
// Mutation failures are non-fatal — reconcile continues.
func applyMutation(
	ctx context.Context,
	kube kubeclient.Interface,
	obj domain.Object,
	resolver *template.Resolver,
	crd types.CRDEntry,
) (*template.Resolver, error) {
	mut := crd.EffectiveMutation()
	if mut == nil || len(mut.Rules) == 0 {
		return resolver, nil
	}

	if calls := mut.ReconcileExternal(); len(calls) > 0 {
		var err error
		resolver, err = external.Run(ctx, crd.GVKString(), resolver, calls, kube.Clientset())
		if err != nil {
			return resolver, err
		}
	}

	_, err := runMutation(ctx, kube, obj, resolver, mut, crd.GVR(), crd.GVKString())
	return resolver, err
}

func runMutation(
	ctx context.Context,
	kube kubeclient.Interface,
	obj domain.Object,
	resolver *template.Resolver,
	cfg *types.MutationConfig,
	gvr schema.GroupVersionResource,
	crdName string,
) (*MutationResult, error) {
	result := &MutationResult{}
	if cfg == nil || len(cfg.Rules) == 0 {
		return result, nil
	}

	data := resolver.Data()
	var err error

	patch := map[string]interface{}{}
	hasPatch := false

	for _, rule := range cfg.Rules {
		if !rule.Fires.FiresAtReconcile() {
			continue
		}
		if !types.EvaluateConditions(data, rule.When, rule.Or, resolver.TemplateEvaluator()) {
			continue
		}

		targetField := rule.Field
		if types.IsTemplate(targetField) {
			if resolved, resolveErr := resolver.Resolve(targetField); resolveErr == nil {
				targetField = resolved
			}
		}

		currentVal, found := types.ResolveScalarField(data, targetField)

		var rawVal interface{}
		var mutationType string
		rawVal, mutationType, err = ResolveRuleValue(rule, found, currentVal, resolver)
		if err != nil {
			return nil, fmt.Errorf("mutation: field %q: %w", targetField, err)
		}
		if rawVal == nil {
			continue
		}

		if fmt.Sprintf("%v", rawVal) == currentVal {
			continue
		}

		setNestedPatch(patch, targetField, rawVal)
		hasPatch = true

		result.Changes = append(result.Changes, MutationChange{
			Field:    targetField,
			OldValue: currentVal,
			NewValue: rawVal,
			Type:     mutationType,
		})

		metrics.RecordMutationFieldDetail(crdName, targetField, mutationType)

		logger.Debug().
			Str("crd", crdName).
			Str("name", obj.GetName()).
			Str("field", targetField).
			Str("old", currentVal).
			Str("new", fmt.Sprintf("%v", rawVal)).
			Str("type", mutationType).
			Msg("mutation: rule applied")
	}

	if !hasPatch {
		return result, nil
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("mutation: marshalling patch: %w", err)
	}

	ns := obj.GetNamespace()
	_, patchErr := kube.DynamicClient().
		Resource(gvr).
		Namespace(ns).
		Patch(ctx, obj.GetName(), k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})

	if patchErr != nil {
		if errors.IsConflict(patchErr) {
			logger.Debug().
				Str("crd", crdName).
				Str("name", obj.GetName()).
				Msg("mutation: resource version conflict — will retry on next reconcile")
			return result, nil
		}
		return nil, fmt.Errorf("mutation: patching %s/%s: %w", ns, obj.GetName(), patchErr)
	}

	result.Applied = len(result.Changes)
	metrics.RecordMutationTotal(crdName)

	logger.Info().
		Str("crd", crdName).
		Str("name", obj.GetName()).
		Int("fieldsChanged", result.Applied).
		Msg("mutation: rules applied")

	return result, nil
}

// ResolveRuleValue determines the value to set for one mutation rule.
// Returns (nil, "", nil) when the rule does not apply.
func ResolveRuleValue(
	rule types.MutationRule,
	found bool,
	currentVal string,
	resolver *template.Resolver,
) (rawVal interface{}, mutationType string, err error) {
	switch {
	case rule.Override != nil:
		mutationType = "override"
		rawVal, err = resolveTypedValue(rule.Override, rule.ValueType, resolver)
	case rule.Default != nil:
		if found && currentVal != "" {
			return nil, "", nil
		}
		mutationType = "default"
		rawVal, err = resolveTypedValue(rule.Default, rule.ValueType, resolver)
	default:
		return nil, "", nil
	}
	return rawVal, mutationType, err
}

func resolveTypedValue(val interface{}, valueType string, resolver *template.Resolver) (interface{}, error) {
	strVal, isStr := val.(string)
	if isStr && types.IsTemplate(strVal) {
		resolved, err := resolver.Resolve(strVal)
		if err != nil {
			return nil, fmt.Errorf("resolving template %q: %w", strVal, err)
		}
		val = resolved
	}

	switch valueType {
	case "int", "integer":
		switch v := val.(type) {
		case int64:
			return v, nil
		case int:
			return int64(v), nil
		case float64:
			return int64(v), nil
		case string:
			i, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to int64: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to int64", val)
		}
	case "bool", "boolean":
		switch v := val.(type) {
		case bool:
			return v, nil
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to bool: %w", v, err)
			}
			return b, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to bool", val)
		}
	case "float", "number":
		switch v := val.(type) {
		case float64:
			return v, nil
		case int64:
			return float64(v), nil
		case int:
			return float64(v), nil
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot convert %q to float64: %w", v, err)
			}
			return f, nil
		default:
			return nil, fmt.Errorf("cannot convert %T to float64", val)
		}
	default:
		if s, ok := val.(string); ok {
			return s, nil
		}
		return fmt.Sprintf("%v", val), nil
	}
}

// setNestedPatch sets value at a dot-notation path (see utils.SplitField) in a
// merge patch.
func setNestedPatch(patch map[string]interface{}, path string, value interface{}) {
	parts := utils.SplitField(path)
	current := patch
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		if _, ok := current[part]; !ok {
			current[part] = map[string]interface{}{}
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			current[part] = map[string]interface{}{}
			next = current[part].(map[string]interface{})
		}
		current = next
	}
}
