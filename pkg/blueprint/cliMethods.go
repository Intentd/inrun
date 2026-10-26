package blueprint

import (
	"fmt"
	"sort"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
)

// DependencyDisplay holds pre-computed dependency graph data for inrun validate --full.
type DependencyDisplay struct {
	StartupOrder []string                     // CRD names in deterministic startup order
	Conditions   map[string]map[string]string // CRD name → dep name → condition string
}

// DependencyDisplayData builds dependency display data for inrun validate --full.
// Returns nil when no enabled CRD declares any dependsOn — callers should skip
// the section entirely in that case.
func (c *Blueprint) DependencyDisplayData() *DependencyDisplay {
	hasDeps := false
	for _, crd := range c.enabledCRDs {
		if len(crd.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}
	if !hasDeps {
		return nil
	}

	dg := NewDependencyGraph(c)
	conditions := make(map[string]map[string]string)
	for name, crd := range c.enabledCRDs {
		if len(crd.DependsOn) == 0 {
			continue
		}
		m := make(map[string]string, len(crd.DependsOn))
		for dep, d := range crd.DependsOn {
			m[dep] = d.Condition
		}
		conditions[name] = m
	}

	return &DependencyDisplay{
		StartupOrder: dg.StartupOrder(),
		Conditions:   conditions,
	}
}

// SortedDependencyNames returns the dependency names for a CRD in sorted order,
// along with their conditions. Used for deterministic display.
func (dd *DependencyDisplay) SortedDeps(crdName string) []string {
	deps := dd.Conditions[crdName]
	if len(deps) == 0 {
		return nil
	}
	names := make([]string, 0, len(deps))
	for dep := range deps {
		names = append(names, dep)
	}
	sort.Strings(names)
	return names
}

func (c *Blueprint) List() map[string]types.CRDEntry {
	return c.Spec.CRDs
}

// All returns every CRD in the blueprint, including disabled ones.
func (c *Blueprint) All() map[string]types.CRDEntry {
	return c.Spec.CRDs
}

// Useful Metadata
func (c *Blueprint) Meta() types.BlueprintMeta {
	return c.metadata
}

// CRDNames returns the names of all enabled CRDs.
func (c *Blueprint) CRDNames() []string {
	names := make([]string, 0, c.Len())
	for name := range c.enabledCRDs {
		names = append(names, name)
	}
	return names
}

// Enabled returns only the enabled CRDs in the blueprint.
func (c *Blueprint) Enabled() map[string]types.CRDEntry {
	if c == nil {
		return nil
	}
	return c.enabledCRDs
}

// Get returns an enabled CRD by name.
func (c *Blueprint) Get(name string) (*types.CRDEntry, error) {
	crd, ok := c.enabledCRDs[name]
	if !ok {
		return nil, fmt.Errorf("crd not found in blueprint")
	}
	return &crd, nil
}

// ToUI returns a UI-friendly representation of the merged Blueprint.
// This method extracts only the fields needed for display in the Console:
//   - API version and kind (always "Blueprint" at runtime)
//   - Metadata (name, description, version, author, license)
//   - All merged CRD definitions
//
// Internal fields (Scheme, GroupVersionKind, etc.) are excluded because they
// have `yaml:"-" json:"-"` tags and won't be serialized to JSON.
//
// This method is used by the /blueprint/raw endpoint to provide a clean,
// readable view of the Blueprint that created the current operator.
func (c *Blueprint) ToUI() *types.BlueprintForUI {

	return &types.BlueprintForUI{
		APIVersion: c.APIVersion,
		Kind:       config.BlueprintKind(),
		Metadata:   c.metadata,
		Spec: types.BlueprintSpecForUI{
			CRDs: c.Spec.CRDs,
		},
	}
}
