package blueprint

import (
	"sort"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// -----------------------------------------------------------------------------
// Index Building
// -----------------------------------------------------------------------------

// BuildLookupIndexes builds O(1) lookup maps indexes after CRDs are loaded.
// All indexes are stored in lowercase for case-insensitive lookups.
// After this runs, the Blueprint supports:
//   - LookupByKind
//   - LookupByTarget
//   - LookupByGVKString
//   - LookupByGVRString
//
// Called last in setGroupVersionKind.
func (c *Blueprint) BuildLookupIndexes() {
	c.apiVersionIndex = make(map[string]string)
	c.kindIndex = make(map[string]string)
	c.nameIndex = make(map[string]string)
	c.pluralIndex = make(map[string]string)
	c.gvkIndex = make(map[string]string)
	c.gvrIndex = make(map[string]string)
	c.targetIndex = make(map[string]string)

	for name, crd := range c.enabledCRDs {
		// Store all keys in lowercase for case-insensitive lookups.
		// apiVersionIndex is keyed by apiVersion+kind concatenated, matching
		// LookupByAPIVersionAndKind's query — apiVersion alone isn't unique
		// when multiple Kinds share a group/version.
		c.apiVersionIndex[strings.ToLower(crd.APIVersion()+crd.Kind())] = name
		c.kindIndex[strings.ToLower(crd.Kind())] = name
		c.nameIndex[strings.ToLower(name)] = name
		if crd.APITypes.Plural != "" {
			c.pluralIndex[strings.ToLower(crd.APITypes.Plural)] = name
		}
		c.gvkIndex[strings.ToLower(crd.GVKString())] = name
		c.gvrIndex[strings.ToLower(crd.GVRString())] = name
		if crd.HasServeTarget() {
			c.targetIndex[strings.ToLower(crd.ServeTarget())] = name
		}
	}
}

// BuildServeEnabledCRDs returns a slice of all serve-enabled CRDs.
// Use for iteration when the map key is not needed.
func (c *Blueprint) BuildServeEnabledCRDs() []*types.CRDEntry {
	if c == nil {
		return nil
	}
	serveEnabled := make([]*types.CRDEntry, 0, c.Len())
	for _, crd := range c.enabledCRDs {
		if crd.ServeEnabled() {
			entry := crd
			serveEnabled = append(serveEnabled, &entry)
		}
	}
	return serveEnabled
}

// ServeEnabledCRDsForCluster returns the serve-enabled CRDs that should be
// present on the named remote cluster. A CRD is included when:
//   - serve.clusters contains clusterName (static match), OR
//   - serve.clusters contains a template expression (resolved at apply time —
//     the CRD could land on any cluster, so include conservatively), OR
//   - any target.clusters entry matches by the same rules above.
//
// CRDs with no serve.clusters (local-only) are never included.
func (c *Blueprint) ServeEnabledCRDsForCluster(clusterName string) []*types.CRDEntry {
	if c == nil {
		return nil
	}
	var out []*types.CRDEntry
	for _, crd := range c.enabledCRDs {
		if !crd.ServeEnabled() || crd.Serve == nil {
			continue
		}
		if crdRoutesToCluster(crd.Serve, clusterName) {
			entry := crd
			out = append(out, &entry)
		}
	}
	return out
}

// crdRoutesToCluster reports whether a serve config routes to the named cluster.
func crdRoutesToCluster(s *types.ServeConfig, clusterName string) bool {
	for _, c := range s.Clusters {
		if types.IsTemplate(c) || c == clusterName {
			return true
		}
	}
	for _, target := range s.Target.Entries {
		for _, c := range target.TargetClusters() {
			if types.IsTemplate(c) || c == clusterName {
				return true
			}
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Lookup Methods (O(1) index-based)
// -----------------------------------------------------------------------------

// LookupByKind finds the CRD entry whose Kind matches the given kind string.
// O(1) lookup using the kind index. Case-insensitive. Nil-safe.
func (c *Blueprint) LookupByKind(kind string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(kind))
	if name, ok := c.kindIndex[key]; ok {
		if entry, ok := c.enabledCRDs[name]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByName finds the CRD entry whose name (the Blueprint map key) matches
// the given name. O(1) — enabledCRDs is already keyed by name. Case-insensitive. Nil-safe.
func (c *Blueprint) LookupByName(name string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if original, ok := c.nameIndex[key]; ok {
		if entry, ok := c.enabledCRDs[original]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByLabel finds the CRD entry whose name (the Blueprint map key) matches
// the given label. Case-sensitive. Nil-safe.
func (c *Blueprint) LookupByLabel(labels map[string]string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	for _, entry := range c.enabledCRDs {
		if entry.Labels.Contains(labels) {
			return &CRDLookupResult{CRD: &entry}
		}
	}

	return nil
}

// LookupByPlural finds the CRD entry whose plural resource name matches the given string.
// O(1) lookup using the plural index. Case-insensitive. Nil-safe.
// Useful for routing requests matched by URL path (e.g. /apis/group/version/pluralresources).
func (c *Blueprint) LookupByPlural(plural string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(plural))
	if original, ok := c.pluralIndex[key]; ok {
		if entry, ok := c.enabledCRDs[original]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByAPIVersionAndKind finds the CRD whose APIVersion and Kind matches the given strings.
// O(1) lookup using the apiVersionIndex.
func (c *Blueprint) LookupByAPIVersionAndKind(apiVersion, kind string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(apiVersion) + strings.TrimSpace(kind))
	if name, ok := c.apiVersionIndex[key]; ok {
		if entry, ok := c.enabledCRDs[name]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByGVKString finds the CRD entry whose GroupVersionKind matches the given GVK string.
// O(1) lookup using the GVK index. Case-insensitive.
func (c *Blueprint) LookupByGVKString(gvkString string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(gvkString))
	if name, ok := c.gvkIndex[key]; ok {
		if entry, ok := c.enabledCRDs[name]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByGVRString finds the CRD entry whose GroupVersionResource matches the given GVR string.
// O(1) lookup using the GVR index. Case-insensitive.
func (c *Blueprint) LookupByGVRString(gvrString string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(gvrString))
	if name, ok := c.gvrIndex[key]; ok {
		if entry, ok := c.enabledCRDs[name]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByTarget finds the CRD entry whose resolved target matches t.
// O(1) lookup using the target index. Case-insensitive.
func (c *Blueprint) LookupByTarget(target string) *CRDLookupResult {
	if c == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(target))
	if name, ok := c.targetIndex[key]; ok {
		if entry, ok := c.enabledCRDs[name]; ok {
			return &CRDLookupResult{CRD: &entry}
		}
	}
	return nil
}

// LookupByTargetOrKind attempts to find a CRD by target first, then by kind.
// Useful for handlers that accept either a target or a kind. Case-insensitive.
func (c *Blueprint) LookupByTargetOrKind(identifier string) *CRDLookupResult {
	identifier = strings.TrimSpace(identifier)
	if r := c.LookupByTarget(identifier); r != nil {
		return r
	}
	return c.LookupByKind(identifier)
}

// MustLookupByTarget finds the CRD entry whose resolved target matches t.
// Panics if not found. Use when you know the target exists. Case-insensitive.
func (c *Blueprint) MustLookupByTarget(target string) *CRDLookupResult {
	r := c.LookupByTarget(target)
	if r == nil {
		panic("CRD not found for target: " + target)
	}
	return r
}

// MustLookupByKind finds the CRD entry whose Kind matches the given kind string.
// Panics if not found. Use when you know the kind exists. Case-insensitive.
func (c *Blueprint) MustLookupByKind(kind string) *CRDLookupResult {
	r := c.LookupByKind(kind)
	if r == nil {
		panic("CRD not found for kind: " + kind)
	}
	return r
}

// -----------------------------------------------------------------------------
// Existence Checks
// -----------------------------------------------------------------------------

// IsKindRegistered returns true if the kind exists in the index.
// Case-insensitive.
func (c *Blueprint) IsKindRegistered(kind string) bool {
	if c == nil {
		return false
	}
	_, ok := c.kindIndex[strings.ToLower(strings.TrimSpace(kind))]
	return ok
}

// IsTargetRegistered returns true if the target exists in the index.
// Case-insensitive.
func (c *Blueprint) IsTargetRegistered(target string) bool {
	if c == nil {
		return false
	}
	_, ok := c.targetIndex[strings.ToLower(strings.TrimSpace(target))]
	return ok
}

// IsGVKRegistered returns true if the GVK exists in the index.
// Case-insensitive.
func (c *Blueprint) IsGVKRegistered(gvkString string) bool {
	if c == nil {
		return false
	}
	_, ok := c.gvkIndex[strings.ToLower(strings.TrimSpace(gvkString))]
	return ok
}

// IsGVRRegistered returns true if the GVR exists in the index.
// Case-insensitive.
func (c *Blueprint) IsGVRRegistered(gvrString string) bool {
	if c == nil {
		return false
	}
	_, ok := c.gvrIndex[strings.ToLower(strings.TrimSpace(gvrString))]
	return ok
}

// -----------------------------------------------------------------------------
// List Methods
// -----------------------------------------------------------------------------

// ListTargets returns all registered targets (lowercase), sorted.
func (c *Blueprint) ListTargets() []string {
	if c == nil {
		return nil
	}
	targets := make([]string, 0, len(c.targetIndex))
	for target := range c.targetIndex {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

// ListKinds returns all registered kinds (lowercase), sorted.
func (c *Blueprint) ListKinds() []string {
	if c == nil {
		return nil
	}
	kinds := make([]string, 0, len(c.kindIndex))
	for kind := range c.kindIndex {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// ListGVKs returns all registered GVK strings (lowercase), sorted.
func (c *Blueprint) ListGVKs() []string {
	if c == nil {
		return nil
	}
	gvks := make([]string, 0, len(c.gvkIndex))
	for gvk := range c.gvkIndex {
		gvks = append(gvks, gvk)
	}
	sort.Strings(gvks)
	return gvks
}

// ListGVRs returns all registered GVR strings (lowercase), sorted.
func (c *Blueprint) ListGVRs() []string {
	if c == nil {
		return nil
	}
	gvrs := make([]string, 0, len(c.gvrIndex))
	for gvr := range c.gvrIndex {
		gvrs = append(gvrs, gvr)
	}
	sort.Strings(gvrs)
	return gvrs
}

// domain.Blueprint Getters

// GetNameByGVKString delegates to LookupByGVKString.
// Used by domain.Blueprint for callers who cannot import blueprint
func (c *Blueprint) GetNameByGVKString(gvkString string) string {
	return c.LookupByGVKString(gvkString).Entry().Name
}

// GetNameByGVRString delegates to LookupByGVRString.
// Used by domain.Blueprint for callers who cannot import blueprint
func (c *Blueprint) GetNameByGVRString(gvrString string) string {
	return c.LookupByGVRString(gvrString).Entry().Name
}

// GetNameByKind delegates to LookupByKind.
// Used by domain.Blueprint for callers who cannot import blueprint
func (c *Blueprint) GetNameByKind(kind string) string {
	return c.LookupByKind(kind).Entry().Name
}

// GetNameByTarget delegates to LookupByGVRString.
// Used by domain.Blueprint for callers who cannot import blueprint
func (c *Blueprint) GetNameByTarget(target string) string {
	return c.LookupByTargetOrAlias(target).Entry().Name
}
