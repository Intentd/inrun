package children

import (
	"github.com/intentd/inrun/pkg/util/kube"

	"github.com/intentd/inrun/domain"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResolveGVR resolves a ManagedResource into a concrete GroupVersionResource.
//
// Explicit fields always win over inference. The ordering below is load-bearing:
// reordering branches changes results for resources that mix explicit and
// inferred fields.
//
//  1. group + version + plural → use as-is
//  2. group + version          → infer plural as lower(kind) + "s"
//  3. apiVersion + plural      → parse apiVersion, use plural
//  4. apiVersion               → parse apiVersion, infer plural
//  5. built-in by kind         → GVRForBuiltIn(kind)
//  6. built-in by plural       → LookupBuiltInByResource(plural)
//  7. custom (kind+group)      → version defaults to "v1"
//
// Returns (GVR{}, false) when no branch applies.
func ResolveGVR(mr domain.ManagedResource) (schema.GroupVersionResource, bool) {
	// 1. Full explicit GVR.
	if mr.Group != "" && mr.Version != "" && mr.Plural != "" {
		return schema.GroupVersionResource{
			Group:    mr.Group,
			Version:  mr.Version,
			Resource: mr.Plural,
		}, true
	}

	// 2. Explicit group + version; infer plural.
	if mr.Group != "" && mr.Version != "" {
		return schema.GroupVersionResource{
			Group:    mr.Group,
			Version:  mr.Version,
			Resource: kube.PluralForKind(mr.Kind),
		}, true
	}

	// 3. APIVersion + plural, with or without a kind.
	if mr.APIVersion != "" && mr.Plural != "" {
		gv, err := schema.ParseGroupVersion(mr.APIVersion)
		if err != nil {
			return schema.GroupVersionResource{}, false
		}
		return gv.WithResource(mr.Plural), true
	}

	// 4. APIVersion + kind; infer plural.
	if mr.APIVersion != "" {
		gvr, err := kube.GVRFromFields(mr.APIVersion, mr.Kind, mr.Plural)
		if err != nil {
			return schema.GroupVersionResource{}, false
		}
		return gvr, true
	}

	// 5. Built-in by kind.
	if gvr, ok := GVRForBuiltInByKind(mr.Kind); ok {
		return gvr, true
	}

	// 6. Built-in by plural resource name (e.g. {group: apps, plural: deployments}).
	if gvr, ok := GVRForBuiltInByResource(mr.Plural); ok {
		return gvr, true
	}

	// 7. Custom resource: require kind + group; default version to "v1".
	if mr.Kind == "" || mr.Group == "" {
		return schema.GroupVersionResource{}, false
	}
	version := mr.Version
	if version == "" {
		version = "v1"
	}
	return schema.GroupVersionResource{
		Group:    mr.Group,
		Version:  version,
		Resource: mr.Kind,
	}, true
}

// ResolveGVK resolves a ManagedResource into a concrete GroupVersionKind.
//
// Explicit fields always win over inference. Unlike ResolveGVR, kind cannot be
// inferred from group+version alone (the built-in registry is keyed by kind and
// by plural, not by GV), so those cases fall through to the built-in lookups.
// The ordering below is load-bearing.
//
//  1. group + version + kind → canonicalize via LookupBuiltInByGVK, else use as-is
//  2. apiVersion + kind      → parse apiVersion, canonicalize, else use as-is
//  3. built-in by kind       → LookupBuiltIn(kind)
//  4. built-in by plural     → LookupBuiltInByResource(plural)
//  5. custom (kind+group)    → version defaults to "v1"
//
// Returns (GVK{}, false) when no branch applies.
func ResolveGVK(mr domain.ManagedResource) (schema.GroupVersionKind, bool) {
	// 1. Full explicit GVK; canonicalize if it matches a built-in.
	if mr.Group != "" && mr.Version != "" && mr.Kind != "" {
		if b, ok := LookupBuiltInByGVK(mr.Group, mr.Version, mr.Kind); ok {
			return schema.GroupVersionKind{
				Group:   b.Group,
				Version: b.Version,
				Kind:    b.Kind,
			}, true
		}
		return schema.GroupVersionKind{
			Group:   mr.Group,
			Version: mr.Version,
			Kind:    mr.Kind,
		}, true
	}

	// 2. APIVersion + kind; canonicalize if it matches a built-in.
	if mr.APIVersion != "" && mr.Kind != "" {
		gvk, err := kube.GVKFromFields(mr.APIVersion, mr.Kind)
		if err != nil {
			return schema.GroupVersionKind{}, false
		}
		if b, ok := LookupBuiltInByGVK(gvk.Group, gvk.Version, gvk.Kind); ok {
			return schema.GroupVersionKind{
				Group:   b.Group,
				Version: b.Version,
				Kind:    b.Kind,
			}, true
		}
		return gvk, true
	}

	// 3. Built-in by kind.
	if mr.Kind != "" {
		res := LookupBuiltIn(mr.Kind)
		if res.found {
			version := mr.Version
			if version == "" {
				version = res.builtIn.Version
			}
			return schema.GroupVersionKind{
				Group:   res.builtIn.Group,
				Version: version,
				Kind:    res.builtIn.Kind,
			}, true
		}
	}

	// 4. Built-in by plural resource name (e.g. {group: apps, plural: deployments}).
	if mr.Plural != "" {
		if b, ok := LookupBuiltInByResource(mr.Plural); ok {
			version := mr.Version
			if version == "" {
				version = b.Version
			}
			return schema.GroupVersionKind{
				Group:   b.Group,
				Version: version,
				Kind:    b.Kind,
			}, true
		}
	}

	// 5. Custom resource: require kind + group; default version to "v1".
	if mr.Kind == "" || mr.Group == "" {
		return schema.GroupVersionKind{}, false
	}
	version := mr.Version
	if version == "" {
		version = "v1"
	}
	return schema.GroupVersionKind{
		Group:   mr.Group,
		Version: version,
		Kind:    mr.Kind,
	}, true
}
