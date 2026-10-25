package children

import (
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ExpandForEachCustomResources expands any CustomResource entries that declare a ForEach.
// It returns a new slice with expanded items. Items without ForEach are copied verbatim.
func ExpandForEachCustomResources(resolver *template.Resolver, srcs []types.CustomResourceTemplateSource) []types.CustomResourceTemplateSource {
	// Fast path: nothing to do
	if !anyHasForEach(len(srcs), func(i int) *types.ForEachSpec { return srcs[i].ForEach }) {
		return srcs
	}

	var result []types.CustomResourceTemplateSource
	for _, src := range srcs {
		// If no ForEach on this item, keep as-is
		if src.ForEach == nil {
			result = append(result, src)
			continue
		}

		// Resolve the items for this ForEach (returns []any or []map[string]any depending on implementation)
		items := resolveForEachItems(resolver.Data(), src.ForEach.Field)
		for i, item := range items {
			ir := itemResolver(resolver, item, src.ForEach.As, i)

			// Start with a shallow copy of src and clear ForEach on the expanded copy
			expanded := src
			expanded.ForEach = nil

			// Resolve simple string fields
			if v, _ := ir.Resolve(src.APIVersion); v != "" {
				expanded.APIVersion = v
			}
			if v, _ := ir.Resolve(src.Kind); v != "" {
				expanded.Kind = v
			}

			// Metadata — resolve all fields into meta before assigning
			meta := src.Metadata // copy
			if v, _ := ir.Resolve(meta.Name); v != "" {
				meta.Name = v
			}
			if v, _ := ir.Resolve(meta.Namespace); v != "" {
				meta.Namespace = v
			}

			// Resolve Labels into meta directly so the assignment below picks them up
			meta.Labels = resolveMap(ir, meta.Labels)

			// Resolve Annotations into meta directly
			meta.Annotations = resolveMap(ir, meta.Annotations)

			expanded.Metadata = meta

			// Resolve spec/status/other maps by walking and resolving string values
			if src.Spec != nil {
				expanded.Spec = resolveMapTemplates(ir, src.Spec)
			}
			if src.Status != nil {
				expanded.Status = resolveMapTemplates(ir, src.Status)
			}
			if src.Other != nil {
				expanded.Other = resolveMapTemplates(ir, src.Other)
			}

			// Reconcile is a bool; if the user templated it as a string, they should have used spec fields.

			// HasStatus pointer: keep original pointer, but if nil and user provided a templated
			// string in some other field to indicate status, leave as-is. (No automatic conversion.)
			expanded.HasStatus = src.HasStatus

			// Append expanded item
			result = append(result, expanded)
		}
	}

	return result
}

// resolveMapTemplates resolves forEach-expanded fields; a value that fails to
// resolve is kept as written.
var resolveMapTemplates = (*template.Resolver).ResolveNestedLenient
