package resources

import (
	"fmt"
	"maps"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/kubewright/core/customresources"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
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
		Other:      nil,
		Observe:    src.Observe,
		ForEach:    src.ForEach,
		When:       src.When,
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
	if resolved.Metadata.Namespace, err = r.ResolveNamespace(src.Metadata.Namespace); err != nil {
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
	if src.Other != nil {
		resolved.Other, err = resolveMapTemplates(r, src.Other)
		if err != nil {
			return resolved, fmt.Errorf("custom.other: %w", err)
		}
	}

	return resolved, nil
}

// resolveMapTemplates resolves a custom resource's fields; the first failure
// is returned.
var resolveMapTemplates = (*template.Resolver).ResolveNested

// CustomResourceSpec builds a customresources.Spec from a CustomResource.
// Template expressions must already be evaluated by template.Resolver before calling.
func CustomResourceSpec(src types.CustomResourceTemplateSource, owner domain.Object) customresources.Spec {
	ownerName := owner.GetName()
	spec := customresources.Spec{
		APIVersion: src.APIVersion,
		Kind:       src.Kind,
		Metadata:   src.Metadata,
		Spec:       src.Spec,
		Other:      src.Other,
	}

	if spec.Metadata.Name == "" {
		spec.Metadata.Name = ownerName + "-custom"
	}

	spec.Metadata.Labels = labels.StampInrunLabels(maps.Clone(spec.Metadata.Labels), owner)
	return spec
}
