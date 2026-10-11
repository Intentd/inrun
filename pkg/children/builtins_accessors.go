// pkg/children/builtins_accessors.go
//
// Accessor functions over the builtInRegistry defined in builtins.go.
// These are the public API for querying built-in kind metadata.
// To add a new built-in kind, edit builtins.go — not this file.
package children

import (
	"sort"
	"strings"

	"github.com/intentd/inrun/domain"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EnrichmentResult holds the result of a built-in lookup.
type EnrichmentResult struct {
	found        bool
	kind         string // canonical PascalCase name (e.g. "Deployment")
	builtIn      BuiltInKind
	displayGroup string // "core" for empty group, otherwise the group string
}

// LookupBuiltIn looks up a Kind in the built-in registry.
// Case-insensitive. Expands shorthands (e.g. "hpa" → "horizontalpodautoscaler").
func LookupBuiltIn(kind string) EnrichmentResult {
	key := strings.ToLower(strings.TrimSpace(kind))
	if key == "" {
		return EnrichmentResult{}
	}
	if expanded, ok := shorthandIndex[key]; ok {
		key = expanded
	}
	b, ok := builtInRegistry[key]
	if !ok {
		return EnrichmentResult{}
	}
	displayGroup := b.Group
	if displayGroup == "" {
		displayGroup = "core"
	}
	return EnrichmentResult{
		found:        true,
		kind:         b.Kind,
		builtIn:      b,
		displayGroup: displayGroup,
	}
}

// Found reports whether the lookup matched a known built-in.
func (r EnrichmentResult) Found() bool { return r.found }

// Kind returns the canonical PascalCase kind name (e.g. "Deployment").
func (r EnrichmentResult) Kind() string { return r.kind }

// DisplayGroup returns the display group: "core" for the empty API group, otherwise the group string.
func (r EnrichmentResult) DisplayGroup() string { return r.displayGroup }

// Group returns the API group (e.g. "apps"; empty string for core resources).
func (r EnrichmentResult) Group() string { return r.builtIn.Group }

// Version returns the API version (e.g. "v1").
func (r EnrichmentResult) Version() string { return r.builtIn.Version }

// Plural returns the plural resource name (e.g. "deployments").
func (r EnrichmentResult) Plural() string { return r.builtIn.Plural }

// Namespaced reports whether the resource is namespace-scoped.
func (r EnrichmentResult) Namespaced() bool { return r.builtIn.Namespaced }

// APIPath returns the API path prefix used in REST URLs (e.g. "apis" or "api").
func (r EnrichmentResult) APIPath() string { return r.builtIn.APIPath }

// LookupBuiltInByGVK looks up a native resource by group, version, and kind.
// Returns the BuiltInKind and true when found. Used to detect custom: declarations
// that reference a type Inrun manages natively (IsChild: true, HookKey set).
// The version check is lenient — it matches on group+kind and requires that the
// canonical version matches, preventing false positives on non-Kubernetes GVKs.
func LookupBuiltInByGVK(group, version, kind string) (BuiltInKind, bool) {
	res := LookupBuiltIn(kind)
	if !res.found {
		return BuiltInKind{}, false
	}
	b := res.builtIn
	if !strings.EqualFold(b.Group, group) {
		return BuiltInKind{}, false
	}
	return b, true
}

// GVRForBuiltIn returns the GroupVersionResource for a built-in kind.
func GVRForBuiltInByKind(kind string) (schema.GroupVersionResource, bool) {
	res := LookupBuiltIn(kind)
	if !res.found {
		return schema.GroupVersionResource{}, false
	}
	b := res.builtIn
	return schema.GroupVersionResource{Group: b.Group, Version: b.Version, Resource: b.Plural}, true
}

// GVKForBuiltIn returns the GroupVersionKind for a built-in kind.
func GVKForBuiltInByKind(kind string) (schema.GroupVersionKind, bool) {
	res := LookupBuiltIn(kind)
	if !res.found {
		return schema.GroupVersionKind{}, false
	}
	b := res.builtIn
	return schema.GroupVersionKind{Group: b.Group, Version: b.Version, Kind: b.Kind}, true
}

// BuiltInMeta returns metadata for a built-in kind. Zero value when unknown.
func BuiltInMeta(kind string) BuiltInKind {
	res := LookupBuiltIn(kind)
	if !res.found {
		return BuiltInKind{}
	}
	return res.builtIn
}

// IsBuiltIn reports whether kind is a known Kubernetes built-in (case-insensitive).
// Also matches plural resource names (e.g. "deployments") so callers that hold
// only a plural — without an explicit kind — get the right answer.
func IsBuiltIn(kind string) bool {
	if LookupBuiltIn(kind).found {
		return true
	}
	_, ok := LookupBuiltInByResource(kind)
	return ok
}

// LookupBuiltInByResource looks up a built-in by singular key, shorthand, or plural
// resource name. Returns (BuiltInKind, true) when found. Used by RBAC generation
// and any caller that works with resource strings rather than Kind names.
func LookupBuiltInByResource(resource string) (BuiltInKind, bool) {
	key := strings.ToLower(strings.TrimSpace(resource))
	if b, ok := builtInRegistry[key]; ok && b.Detect != nil {
		return b, true
	}
	if expanded, ok := shorthandIndex[key]; ok {
		if b, ok := builtInRegistry[expanded]; ok && b.Detect != nil {
			return b, true
		}
	}
	for _, b := range builtInRegistry {
		if b.Plural == key && b.Detect != nil {
			return b, true
		}
	}
	return BuiltInKind{}, false
}

// GVRForBuiltInByResource returns the GroupVersionResource for a built-in
// resource string (singular, plural, or shorthand).
func GVRForBuiltInByResource(resource string) (schema.GroupVersionResource, bool) {
	b, ok := LookupBuiltInByResource(resource)
	if !ok {
		return schema.GroupVersionResource{}, false
	}
	return schema.GroupVersionResource{Group: b.Group, Version: b.Version, Resource: b.Plural}, true
}

// GVKForBuiltInByResource returns the GroupVersionKind for a built-in
// resource string (singular, plural, or shorthand).
func GVKForBuiltInByResource(resource string) (schema.GroupVersionKind, bool) {
	b, ok := LookupBuiltInByResource(resource)
	if !ok {
		return schema.GroupVersionKind{}, false
	}
	return schema.GroupVersionKind{Group: b.Group, Version: b.Version, Kind: b.Kind}, true
}

// ToManagedResourceByKind converts a builtin resource to a domain.ManagedResource
// definition using kind.
func ToManagedResourceByKind(kind string) domain.ManagedResource {
	res := LookupBuiltIn(kind)
	if !res.found {
		return domain.ManagedResource{}
	}
	return domain.ManagedResource{
		APIVersion: res.builtIn.APIVersion().String(),
		Kind:       res.builtIn.Kind,
		Group:      res.builtIn.Group,
		Version:    res.builtIn.Version,
		Plural:     res.builtIn.Plural,
	}
}

// ToManagedResourceByResource converts a builtin resource to a domain.ManagedResource
// deletion using resource.
func ToManagedResourceByResource(resource string) domain.ManagedResource {
	res, ok := LookupBuiltInByResource(resource)
	if !ok {
		return domain.ManagedResource{}
	}
	return domain.ManagedResource{
		APIVersion: res.APIVersion().String(),
		Kind:       res.Kind,
		Group:      res.Group,
		Version:    res.Version,
		Plural:     res.Plural,
	}
}

// AllBuiltInKinds returns all canonical Kind names, sorted alphabetically.
func AllBuiltInKinds() []string {
	kinds := make([]string, 0, len(builtInRegistry))
	for k, b := range builtInRegistry {
		if strings.Contains(k, "_") {
			continue // skip internal alias keys like "event_events"
		}
		kinds = append(kinds, b.Kind)
	}
	for i := 0; i < len(kinds); i++ {
		for j := i + 1; j < len(kinds); j++ {
			if kinds[i] > kinds[j] {
				kinds[i], kinds[j] = kinds[j], kinds[i]
			}
		}
	}
	return kinds
}

// AllBuiltInKindDefs returns all entries from the built-in registry.
// Entries with a nil Detect field represent aliases or internal entries without
// RBAC detection logic. Callers that need only detectable entries should filter
// on Detect != nil.
func AllBuiltInKindDefs() []BuiltInKind {
	result := make([]BuiltInKind, 0, len(builtInRegistry))
	for _, b := range builtInRegistry {
		result = append(result, b)
	}
	return result
}

// enrichmentGroups maps each canonical built-in name to its full list of valid
// enrichment identifiers (name, plural, shorthands, synthetic aliases).
// Computed once at init from the immutable builtInRegistry.
var enrichmentGroups map[string][]string

// enrichmentIndex is the reverse: any valid identifier → canonical name.
// Used for O(1) lookups in enrichmentEnabled and IsValidEnrichmentTarget.
var enrichmentIndex map[string]string

func init() {
	enrichmentGroups = buildEnrichmentGroups()
	idx := make(map[string]string)
	for canonical, aliases := range enrichmentGroups {
		for _, a := range aliases {
			idx[a] = canonical
		}
	}
	enrichmentIndex = idx
}

// buildEnrichmentGroups constructs a map where each canonical built-in name
// maps to the list of all valid enrichment identifiers for that resource.
// Reads from enrichmentMeta (in builtins.go) for target/key config, and from
// builtInRegistry for plural and shorthand aliases.
func buildEnrichmentGroups() map[string][]string {
	groups := make(map[string][]string)

	for name, em := range enrichmentMeta {
		if !em.Target {
			continue
		}

		var list []string
		list = append(list, name)

		if b, ok := builtInRegistry[name]; ok {
			if b.Plural != "" {
				list = append(list, b.Plural)
			}
			for _, s := range b.Shorthands {
				list = append(list, strings.ToLower(s))
			}
		}

		for _, a := range em.EnrichKeys {
			list = append(list, a)
		}

		sort.Strings(list)
		groups[name] = list
	}

	return groups
}

// SupportedEnrichmentGroups returns all supported enrichment targets, including
// built-in Kubernetes resources and synthetic Inrun-only targets.
func SupportedEnrichmentGroups() map[string][]string {
	return enrichmentGroups
}

// IsValidEnrichmentTarget reports whether the given name is a supported
// context-enrichment target.
func IsValidEnrichmentTarget(name string) bool {
	name = strings.ToLower(name)
	if name == "" {
		return false
	}
	_, ok := enrichmentIndex[name]
	return ok
}

// ListKindsForFakeDynamic returns a GVR→list-kind map for every registered
// built-in, suitable for fakedynamic.NewSimpleDynamicClientWithCustomListKinds.
//
// The fake dynamic client refuses to List a GVR whose list kind was not
// registered at construction. Registering the whole registry up front lets a
// reconciler List any built-in it might read, whether or not the fixture
// seeded one — which is the case for reads like "list ConfigMaps to decide
// whether to create one".
func ListKindsForFakeDynamic() map[schema.GroupVersionResource]string {
	out := make(map[schema.GroupVersionResource]string, len(builtInRegistry))
	for _, b := range builtInRegistry {
		gvr := schema.GroupVersionResource{
			Group:    b.Group,
			Version:  b.Version,
			Resource: b.Plural,
		}
		out[gvr] = b.Kind + "List"
	}
	return out
}

// ── Readiness / deletion-protection queries ───────────────────────────────────

func SkipObservedGenerationGVKs() []string {
	return gvksByFlag(func(b BuiltInKind) bool { return b.SkipObservedGeneration })
}

func SkipStatusSubresourceGVKs() []string {
	return gvksByFlag(func(b BuiltInKind) bool { return b.SkipStatusSubresource })
}

func StatuslessGVKs() []string {
	return gvksByFlag(func(b BuiltInKind) bool { return b.Statusless })
}

func gvksByFlag(predicate func(BuiltInKind) bool) []string {
	var out []string
	for key, b := range builtInRegistry {
		if !predicate(b) {
			continue
		}
		kind := b.Kind
		if kind == "" {
			kind = strings.ToUpper(key[:1]) + key[1:]
		}
		if b.Group == "" {
			out = append(out, b.Version+"/"+kind)
		} else {
			out = append(out, b.Group+"/"+b.Version+"/"+kind)
		}
	}
	return out
}
