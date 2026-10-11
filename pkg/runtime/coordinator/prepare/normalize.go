package prepare

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyNormalize applies the CRD's normalize.spec templates to a deep copy of
// the CR, returning the normalized copy for all downstream reconcile steps.
//
// Works for both typed and unstructured objects via the resolver's ObjectToMap
// path — no guard needed for unstructured mode. The returned object is a
// deep copy wrapped as *unstructured.Unstructured so downstream steps have a
// consistent content map to template against.
//
// Returns the original object unchanged when no normalize block is declared.
func applyNormalize(
	ctx context.Context,
	crd types.CRDEntry,
	obj domain.Object,
) (domain.Object, *template.Resolver, []types.NormalizeChange, error) {
	norm := crd.EffectiveNormalize()
	if norm == nil || len(norm.Spec) == 0 {
		baseResolver, err := template.NewResolver(ctx, obj)
		if err != nil {
			return obj, nil, nil, fmt.Errorf("normalize: building base resolver: %w", err)
		}
		return obj, baseResolver, nil, nil
	}

	log := logger.FromContext(ctx)

	// Deep-copy so the informer cache object is never modified.
	cloned := obj.DeepCopyObject().(domain.Object)

	baseResolver, err := template.NewResolver(ctx, cloned)
	if err != nil {
		return obj, nil, nil, fmt.Errorf("normalize: building resolver: %w", err)
	}

	// Get content map — works for both typed and unstructured via ObjectToMap.
	// For unstructured, this is a direct map reference.
	// For typed, ObjectToMap produces the equivalent map via JSON round-trip.
	content, err := template.ObjectToMap(cloned)
	if err != nil {
		return obj, nil, nil, fmt.Errorf("normalize: reading object content: %w", err)
	}
	if content == nil {
		content = map[string]interface{}{}
	}

	audit := norm.Audit
	var changes []types.NormalizeChange

	for fieldPath, tpl := range norm.Spec {
		rendered, err := baseResolver.Resolve(tpl)
		if err != nil {
			return obj, nil, nil, fmt.Errorf("normalize spec.%s: %w", fieldPath, err)
		}
		rendered = strings.TrimSpace(rendered)

		var parsed interface{}
		if declaredType, ok := norm.Types[fieldPath]; ok {
			parsed, err = coerceNormalizedValue(rendered, declaredType)
			if err != nil {
				return obj, nil, nil, fmt.Errorf("normalize spec.%s: type %q: %w", fieldPath, declaredType, err)
			}
		} else {
			parsed = parseNormalizedValue(rendered)
		}

		if parsed == nil {
			log.Debug().Str("field", fieldPath).Msg("normalize: field omitted (nil)")
			continue
		}

		fullPath := "spec." + fieldPath
		var before interface{}
		if audit {
			before = nestedGet(content, fullPath)
		}
		if err := setNestedNormalized(content, fullPath, parsed); err != nil {
			return obj, nil, nil, fmt.Errorf("normalize spec.%s: setting field: %w", fieldPath, err)
		}
		if audit && fmt.Sprintf("%v", before) != fmt.Sprintf("%v", parsed) {
			changes = append(changes, types.NormalizeChange{
				Field: fieldPath,
				From:  before,
				To:    parsed,
			})
		}
		log.Debug().
			Str("field", fieldPath).
			Str("rendered", rendered).
			Msg("normalize: field normalized")
	}

	// Wrap the mutated content as *unstructured.Unstructured so downstream
	// steps have a consistent surface regardless of whether input was typed.
	normalized := &unstructured.Unstructured{Object: content}

	normalizedResolver, err := template.NewResolver(ctx, normalized)
	if err != nil {
		return obj, nil, nil, fmt.Errorf("normalize: building normalized resolver: %w", err)
	}

	return normalized, normalizedResolver, changes, nil
}

func parseNormalizedValue(s string) interface{} {
	if s == "" {
		return ""
	}
	var v interface{}
	if err := yaml.Unmarshal([]byte(s), &v); err == nil && v != nil {
		switch t := v.(type) {
		case int:
			return int64(t)
		case nil:
			return ""
		default:
			return t
		}
	}
	return s
}

func coerceNormalizedValue(rendered, typ string) (interface{}, error) {
	if rendered == "" {
		return nil, nil
	}
	switch strings.ToLower(typ) {
	case "int", "integer":
		i, err := strconv.ParseInt(rendered, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %q to int: %w", rendered, err)
		}
		return i, nil
	case "bool", "boolean":
		b, err := strconv.ParseBool(rendered)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %q to bool: %w", rendered, err)
		}
		return b, nil
	case "float", "number":
		f, err := strconv.ParseFloat(rendered, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %q to float: %w", rendered, err)
		}
		return f, nil
	case "string", "":
		return rendered, nil
	default:
		return nil, fmt.Errorf("unknown type %q — use int, bool, float, or string", typ)
	}
}

func setNestedNormalized(obj map[string]interface{}, path string, value interface{}) error {
	parts := splitDotPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}
	current := obj
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			newMap := map[string]interface{}{}
			current[part] = newMap
			current = newMap
			continue
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("path segment %q is not a map (got %T)", part, next)
		}
		current = nextMap
	}
	current[parts[len(parts)-1]] = value
	return nil
}

func nestedGet(obj map[string]interface{}, path string) interface{} {
	parts := splitDotPath(path)
	current := obj
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part]
		if !ok {
			return nil
		}
		m, ok := next.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m
	}
	return current[parts[len(parts)-1]]
}

func splitDotPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	return append(parts, path[start:])
}
