package resources

import (
	"fmt"
	"maps"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/kubewright/core/customresources"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/structured"
)

// ResolveCustomResource resolves all template expressions in a
// CustomResourceTemplateSource. It:
// - resolves simple string fields via r.Resolve
// - resolves labels/annotations via r.ResolveLabels / r.ResolveAnnotations
// - walks spec/status/other maps and resolves any string values
// - preserves non-string typed values
//
// The returned value is the same concrete template type with all string
// templates evaluated. The caller then calls customresources.Resolve to
// convert the resolved template into a customresources.Spec.
func ResolveCustomResource(r *template.Resolver, src types.CustomResourceTemplateSource) (types.CustomResourceTemplateSource, error) {
	resolved := types.CustomResourceTemplateSource{
		APIVersion: src.APIVersion,
		Kind:       src.Kind,
		Metadata:   src.Metadata, // we'll overwrite fields below as needed
		Spec:       nil,
		Status:     nil,
		Other:      nil,
		HasStatus:  src.HasStatus,
		Reconcile:  src.Reconcile,
		ForEach:    src.ForEach,
		Conditions: src.Conditions,
		Or:         src.Or,
	}

	var err error

	// apiVersion / kind
	if resolved.APIVersion, err = r.Resolve(src.APIVersion); err != nil {
		return resolved, fmt.Errorf("custom.apiVersion: %w", err)
	}
	if resolved.Kind, err = r.Resolve(src.Kind); err != nil {
		return resolved, fmt.Errorf("custom.kind: %w", err)
	}

	// metadata.name / namespace / sleep
	if resolved.Metadata.Name, err = r.Resolve(src.Metadata.Name); err != nil {
		return resolved, fmt.Errorf("custom.metadata.name: %w", err)
	}

	// Namespace defaulting: if template omitted, default to owner namespace at runtime.
	ns := src.Metadata.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Metadata.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("custom.metadata.namespace: %w", err)
	}

	// Labels / Annotations (helpers return map[string]string)
	if resolved.Metadata.Labels, err = r.ResolveMap(src.Metadata.Labels); err != nil {
		return resolved, fmt.Errorf("custom.metadata.labels: %w", err)
	}
	if resolved.Metadata.Annotations, err = r.ResolveMap(src.Metadata.Annotations); err != nil {
		return resolved, fmt.Errorf("custom.metadata.annotations: %w", err)
	}

	// Resolve spec/status/other by walking maps and resolving string values.
	if src.Spec != nil {
		resolved.Spec, err = resolveMapTemplates(r, src.Spec)
		if err != nil {
			return resolved, fmt.Errorf("custom.spec: %w", err)
		}
	}
	if src.Status != nil {
		resolved.Status, err = resolveMapTemplates(r, src.Status)
		if err != nil {
			return resolved, fmt.Errorf("custom.status: %w", err)
		}
	}
	if src.Other != nil {
		resolved.Other, err = resolveMapTemplates(r, src.Other)
		if err != nil {
			return resolved, fmt.Errorf("custom.other: %w", err)
		}
	}

	return resolved, nil
}

// resolveMapTemplates walks a map[string]any and resolves any string values
// using the Resolver. It recurses into nested maps and slices. Returns a new
// map[string]any with resolved values.
func resolveMapTemplates(r *template.Resolver, in map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		rv, err := resolveValueTemplates(r, v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = rv
	}
	return out, nil
}

// resolveValueTemplates resolves a single value. If it's a string, run through
// r.Resolve. If it's a map[string]any, recurse. If it's a slice, resolve each
// element. Otherwise return as-is.
func resolveValueTemplates(r *template.Resolver, v any) (any, error) {
	switch vv := v.(type) {
	case string:
		res, err := r.Resolve(vv)
		if err != nil {
			return nil, err
		}
		if res == "" {
			return vv, nil
		}
		// When a template expression was resolved, try to coerce the result to a
		// native type so integer/boolean/JSON CRD fields pass API server validation.
		if types.IsTemplate(vv) {
			return structured.TryCoerceString(res), nil
		}
		return res, nil

	case map[string]any:
		return resolveMapTemplates(r, vv)

	case []any:
		out := make([]any, 0, len(vv))
		for _, e := range vv {
			re, err := resolveValueTemplates(r, e)
			if err != nil {
				return nil, err
			}
			out = append(out, re)
		}
		return out, nil

	// Preserve typed maps like map[string]string by converting and resolving strings
	default:
		// attempt to detect map[string]string and convert
		// keep simple: if it's map[string]string, convert to map[string]any and resolve
		if m, ok := vv.(map[string]string); ok {
			out := make(map[string]any, len(m))
			for kk, vv2 := range m {
				res, err := r.Resolve(vv2)
				if err != nil {
					return nil, err
				}
				if res == "" {
					out[kk] = vv2
				} else {
					out[kk] = res
				}
			}
			return out, nil
		}
		return v, nil
	}
}

// CustomResourceSpec builds a customresources.Spec from a CustomResource.
// Template expressions must already be evaluated by template.Resolver before calling.
func CustomResourceSpec(src types.CustomResourceTemplateSource, owner domain.Object) customresources.Spec {
	ownerName := owner.GetName()
	spec := customresources.Spec{
		APIVersion: src.APIVersion,
		Kind:       src.Kind,
		Metadata:   src.Metadata,
		Spec:       src.Spec,
		Status:     src.Status,
		Other:      src.Other,
		HasStatus:  src.HasStatus,
		Reconcile:  src.Reconcile,
	}

	if spec.Metadata.Name == "" {
		spec.Metadata.Name = ownerName + "-custom"
	}

	spec.Metadata.Labels = labels.StampInrunLabels(maps.Clone(spec.Metadata.Labels), owner.GetName(), owner.GetAnnotations())
	return spec
}
