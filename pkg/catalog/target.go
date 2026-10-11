package catalog

import (
	"sort"

	"github.com/intentd/inrun/pkg/types"
)

// CRDLookupResult carries the resolved CRD entry and the alias name that was
// used to find it. Alias is empty when a primary target or a non-alias lookup matched.
type CRDLookupResult struct {
	CRD   *types.CRDEntry
	Alias string
}

// Entry returns the resolved CRD entry. Nil-safe — returns nil when the
// lookup result itself is nil, so callers can write:
//
//	crd := cat.LookupByKind("App").Entry()
//	if crd == nil { ... }
func (r *CRDLookupResult) Entry() *types.CRDEntry {
	if r == nil || r.CRD == nil {
		return nil
	}
	return r.CRD
}

// LookupByTargetOrAlias finds a serve-enabled CRD by primary target or alias name.
// Returns nil when no match is found or when the matching entry is disabled.
//
// Primary targets are checked before aliases. When the same name is both a
// primary target on one CRD and an alias on another, the primary target wins —
// inrun validate catches this collision at load time.
func (c *Catalog) LookupByTargetOrAlias(target string) *CRDLookupResult {
	if c == nil || target == "" {
		return nil
	}
	// 1. Primary targets first — only when the primary surface is enabled.
	for i := range c.enabledCRDs {
		crd := c.enabledCRDs[i]
		if crd.HasServeTarget() && crd.ServeTarget() == target {
			return &CRDLookupResult{CRD: &crd, Alias: ""}
		}
	}
	// 2. Alias entries (non-primary enabled entries in Target.Entries).
	for i := range c.enabledCRDs {
		crd := c.enabledCRDs[i]
		if !crd.ServeEnabled() || crd.Serve == nil {
			continue
		}
		cfg, ok := crd.Serve.Target.Entries[target]
		if !ok || cfg == nil || cfg.Primary || !cfg.IsEnabled() {
			continue
		}
		return &CRDLookupResult{CRD: &crd, Alias: target}
	}
	return nil
}

// LookupByKindOrAlias finds a CRD by Kubernetes Kind first, then falls back to
// LookupByTargetOrAlias (primary target, then alias). Use this when the caller
// may send any form of identifier — Kubernetes Kind, serve target name, or alias.
func (c *Catalog) LookupByKindOrAlias(identifier string) *CRDLookupResult {
	if r := c.LookupByKind(identifier); r != nil {
		return r
	}
	return c.LookupByTargetOrAlias(identifier)
}

// AvailableTargets returns the primary targets and alias names of all
// serve-enabled CRDs, sorted alphabetically.
// Used in error messages ("available: app, internal, public, smartapp, v2").
func (c *Catalog) AvailableTargets() []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]struct{}, c.Len()*2)
	for _, crd := range c.enabledCRDs {
		if crd.HasServeTarget() {
			seen[crd.ServeTarget()] = struct{}{}
		}
		for name, cfg := range crd.AllServeTargets() {
			if cfg != nil && !cfg.Primary && cfg.IsEnabled() {
				seen[name] = struct{}{}
			}
		}
	}
	targets := make([]string, 0, len(seen))
	for t := range seen {
		targets = append(targets, t)
	}
	sort.Strings(targets)
	return targets
}

// ServeCatalog returns all serve-enabled CRD entries, sorted by target.
// Used by the catalog endpoint to list available services.
func (c *Catalog) ServeCatalog() []*types.CRDEntry {
	if c == nil {
		return nil
	}
	catalog := make([]*types.CRDEntry, 0, c.Len())
	for _, crd := range c.enabledCRDs {
		if crd.HasServeTarget() {
			catalog = append(catalog, &crd)
		}
	}
	sort.Slice(catalog, func(i, j int) bool {
		return catalog[i].ServeTarget() < catalog[j].ServeTarget()
	})
	return catalog
}

// ServeEnabledCRDs returns all serve-enabled CRD entries as a slice.
// Uses cached serveEnabledCRDs if available, otherwise builds it.
func (c *Catalog) ServeEnabledCRDs() []*types.CRDEntry {
	if c == nil {
		return nil
	}
	if c.serveEnabledCRDs == nil {
		return c.BuildServeEnabledCRDs()
	}
	return c.serveEnabledCRDs
}
