package catalog

import (
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
)

// NewCatalogWithLifecycleForTest builds a minimal Catalog with the lifecycle
// block set, for use in validate package tests that cannot set the unexported field.
func NewCatalogWithLifecycleForTest(kind string, lc *types.CatalogLifecycle) *Catalog {
	return &Catalog{Kind: kind, lifecycle: lc}
}

// NewCatalogWithPolicyForTest builds a minimal Catalog with the policy block set.
func NewCatalogWithPolicyForTest(kind string, p *types.CatalogPolicy) *Catalog {
	return &Catalog{Kind: kind, policy: p}
}

// NewCatalogWithMetadataForTest builds a Catalog with a specific metadata block
// and an optional enabledCRDs map. Nil crds is treated as empty.
func NewCatalogWithMetadataForTest(m types.CatalogMeta, crds map[string]types.CRDEntry) *Catalog {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	return &Catalog{metadata: m, enabledCRDs: crds}
}

// wireForTest runs the same setGroupVersionKind → setDefaults sequence
// Validate() runs in production (see validate.go), so a Catalog built by
// NewCatalogForTest/NewFromEntryPointers is immediately usable by
// LookupByKind/LookupByName/GVR/etc. without every test remembering to call
// BuildLookupIndexes (or hitting Kind()/GVR() silently returning zero values
// because GroupVersionKind/GroupVersionResource were never computed).
// Panics on error — these are hand-authored test fixtures, not untrusted
// input, so a malformed one should fail loudly and immediately.
func (c *Catalog) wireForTest() *Catalog {
	// setGroupVersionKind is best-effort: graph-only fixtures (e.g. cycle
	// detection tests) legitimately omit apiTypes, so we skip rather than panic.
	_ = c.SetGroupVersionKind()
	if err := c.SetDefaults(config.NewDefaultConfig()); err != nil {
		panic("catalog test fixture: " + err.Error())
	}
	// Pre-build the serve-enabled cache so ServeEnabledCRDs() returns the
	// correct slice without requiring a full BuildExpanded/ParseFile call.
	c.serveEnabledCRDs = c.BuildServeEnabledCRDs()
	return c
}

// NewCatalogForTest creates a Catalog with pre-set enabledCRDs for testing.
// Bypasses YAML parsing and Validate()'s other steps (uniqueness,
// dependsOn, reconciler mode, …) but still wires GVK/GVR/lookup indexes and
// defaults exactly as Validate() would, so lookups and field defaults
// behave the same as a fully loaded Catalog.
func NewCatalogForTest(crds map[string]types.CRDEntry) *Catalog {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	for key, entry := range crds {
		if entry.Name == "" {
			entry.Name = key
			crds[key] = entry
		}
	}
	return (&Catalog{enabledCRDs: crds}).wireForTest()
}

// NewCatalogForTestWithSpec creates a wired Catalog with pre-set CRDs and a Spec.
// Use when the test needs spec-level fields (e.g. Finalizers) applied during the
// initial SetDefaults pass that wireForTest runs.
func NewCatalogForTestWithSpec(crds map[string]types.CRDEntry, spec types.CatalogSpec) *Catalog {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	for key, entry := range crds {
		if entry.Name == "" {
			entry.Name = key
			crds[key] = entry
		}
	}
	return (&Catalog{enabledCRDs: crds, Spec: spec}).wireForTest()
}

// SetCatalogDirForTest sets the catalogDir field on a Catalog for use in tests
// that exercise publish-path validation which reads files relative to that directory.
func (c *Catalog) SetCatalogDirForTest(dir string) { c.catalogDir = dir }

// SetEnabledCRDsForTest replaces the enabledCRDs map on an existing Catalog.
// Allows tests that build up a Catalog in stages to set CRDs after construction.
func (c *Catalog) SetEnabledCRDsForTest(crds map[string]types.CRDEntry) {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	c.enabledCRDs = crds
}

// NewFromEntryPointers creates a Catalog from a map of CRD entry pointers.
// Useful when you have pointers and want to avoid copying. See
// NewCatalogForTest for what gets wired.
func NewFromEntryPointers(entries map[string]*types.CRDEntry) *Catalog {
	if entries == nil {
		entries = map[string]*types.CRDEntry{}
	}
	cat := &Catalog{
		enabledCRDs: make(map[string]types.CRDEntry, len(entries)),
	}
	for k, v := range entries {
		if v != nil {
			entry := *v
			if entry.Name == "" {
				entry.Name = k
			}
			cat.enabledCRDs[k] = entry
		}
	}
	return cat.wireForTest()
}
