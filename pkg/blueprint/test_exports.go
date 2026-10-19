package blueprint

import (
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
)

// NewBlueprintWithLifecycleForTest builds a minimal Blueprint with the lifecycle
// block set, for use in validate package tests that cannot set the unexported field.
func NewBlueprintWithLifecycleForTest(kind string, lc *types.BlueprintLifecycle) *Blueprint {
	return &Blueprint{Kind: kind, lifecycle: lc}
}

// NewBlueprintWithPolicyForTest builds a minimal Blueprint with the policy block set.
func NewBlueprintWithPolicyForTest(kind string, p *types.BlueprintPolicy) *Blueprint {
	return &Blueprint{Kind: kind, policy: p}
}

// NewBlueprintWithMetadataForTest builds a Blueprint with a specific metadata block
// and an optional enabledCRDs map. Nil crds is treated as empty.
func NewBlueprintWithMetadataForTest(m types.BlueprintMeta, crds map[string]types.CRDEntry) *Blueprint {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	return &Blueprint{metadata: m, enabledCRDs: crds}
}

// wireForTest runs the same setGroupVersionKind → setDefaults sequence
// Validate() runs in production (see validate.go), so a Blueprint built by
// NewBlueprintForTest/NewFromEntryPointers is immediately usable by
// LookupByKind/LookupByName/GVR/etc. without every test remembering to call
// BuildLookupIndexes (or hitting Kind()/GVR() silently returning zero values
// because GroupVersionKind/GroupVersionResource were never computed).
// Panics on error — these are hand-authored test fixtures, not untrusted
// input, so a malformed one should fail loudly and immediately.
func (c *Blueprint) wireForTest() *Blueprint {
	// setGroupVersionKind is best-effort: graph-only fixtures (e.g. cycle
	// detection tests) legitimately omit apiTypes, so we skip rather than panic.
	_ = c.SetGroupVersionKind()
	if err := c.SetDefaults(config.NewDefaultConfig()); err != nil {
		panic("blueprint test fixture: " + err.Error())
	}
	// Pre-build the serve-enabled cache so ServeEnabledCRDs() returns the
	// correct slice without requiring a full BuildExpanded/ParseFile call.
	c.serveEnabledCRDs = c.BuildServeEnabledCRDs()
	return c
}

// NewBlueprintForTest creates a Blueprint with pre-set enabledCRDs for testing.
// Bypasses YAML parsing and Validate()'s other steps (uniqueness,
// dependsOn, reconciler mode, …) but still wires GVK/GVR/lookup indexes and
// defaults exactly as Validate() would, so lookups and field defaults
// behave the same as a fully loaded Blueprint.
func NewBlueprintForTest(crds map[string]types.CRDEntry) *Blueprint {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	for key, entry := range crds {
		if entry.Name == "" {
			entry.Name = key
			crds[key] = entry
		}
	}
	return (&Blueprint{enabledCRDs: crds}).wireForTest()
}

// NewBlueprintForTestWithSpec creates a wired Blueprint with pre-set CRDs and a Spec.
// Use when the test needs spec-level fields (e.g. Finalizers) applied during the
// initial SetDefaults pass that wireForTest runs.
func NewBlueprintForTestWithSpec(crds map[string]types.CRDEntry, spec types.BlueprintSpec) *Blueprint {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	for key, entry := range crds {
		if entry.Name == "" {
			entry.Name = key
			crds[key] = entry
		}
	}
	return (&Blueprint{enabledCRDs: crds, Spec: spec}).wireForTest()
}

// SetBlueprintDirForTest sets the blueprintDir field on a Blueprint for use in tests
// that exercise publish-path validation which reads files relative to that directory.
func (c *Blueprint) SetBlueprintDirForTest(dir string) { c.blueprintDir = dir }

// SetEnabledCRDsForTest replaces the enabledCRDs map on an existing Blueprint.
// Allows tests that build up a Blueprint in stages to set CRDs after construction.
func (c *Blueprint) SetEnabledCRDsForTest(crds map[string]types.CRDEntry) {
	if crds == nil {
		crds = map[string]types.CRDEntry{}
	}
	c.enabledCRDs = crds
}

// NewFromEntryPointers creates a Blueprint from a map of CRD entry pointers.
// Useful when you have pointers and want to avoid copying. See
// NewBlueprintForTest for what gets wired.
func NewFromEntryPointers(entries map[string]*types.CRDEntry) *Blueprint {
	if entries == nil {
		entries = map[string]*types.CRDEntry{}
	}
	bp := &Blueprint{
		enabledCRDs: make(map[string]types.CRDEntry, len(entries)),
	}
	for k, v := range entries {
		if v != nil {
			entry := *v
			if entry.Name == "" {
				entry.Name = k
			}
			bp.enabledCRDs[k] = entry
		}
	}
	return bp.wireForTest()
}
