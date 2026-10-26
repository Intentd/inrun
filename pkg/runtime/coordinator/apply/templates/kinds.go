package templates

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/types"
)

// Kinds returns the resources a CRD's templates can write, under any serve
// target: the built-in kinds their onCreate and onReconcile blocks declare,
// and their custom resources, resolved with mapper (one the mapper doesn't
// know, or with a templated apiVersion or kind, is left out). Cleanup lists
// these kinds to find the CR's template children.
func Kinds(crd types.CRDEntry, mapper meta.RESTMapper) []schema.GroupVersionResource {
	seen := map[schema.GroupVersionResource]bool{}
	var out []schema.GroupVersionResource
	add := func(gvr schema.GroupVersionResource) {
		if !seen[gvr] {
			seen[gvr] = true
			out = append(out, gvr)
		}
	}

	boxes := []*types.OperatorBoxConfig{crd.EffectiveOperatorBox("")}
	if crd.HasServeTargetEntries() {
		for target := range crd.Serve.Target.Entries {
			boxes = append(boxes, crd.EffectiveOperatorBox(target))
		}
	}

	for _, box := range boxes {
		for kind := range box.DeclaredChildKinds() {
			if gvr, ok := children.GVRForBuiltInByKind(kind); ok {
				add(gvr)
			}
		}
		if mapper == nil || box == nil {
			continue
		}
		for _, ht := range []*types.HookTemplates{box.EffectiveOnCreate(), box.EffectiveOnReconcile()} {
			if ht == nil {
				continue
			}
			ht.VisitResources(func(res interface{}) {
				cr, ok := res.(types.CustomResourceTemplateSource)
				if !ok || strings.Contains(cr.APIVersion+cr.Kind, "{{") {
					return
				}
				gv, err := schema.ParseGroupVersion(cr.APIVersion)
				if err != nil {
					return
				}
				m, err := mapper.RESTMapping(gv.WithKind(cr.Kind).GroupKind(), gv.Version)
				if err != nil {
					return
				}
				add(m.Resource)
			})
		}
	}
	return out
}
