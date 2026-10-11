package catalog

import (
	"fmt"
	"sort"
	"strings"

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
func (c *Catalog) DependencyDisplayData() *DependencyDisplay {
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

func (c *Catalog) List() map[string]types.CRDEntry {
	return c.Spec.CRDs
}

// All returns every CRD in the catalog, including disabled ones.
func (c *Catalog) All() map[string]types.CRDEntry {
	return c.Spec.CRDs
}

// Useful Metadata
func (c *Catalog) Meta() types.CatalogMeta {
	return c.metadata
}

// Exists returns true if a CRD with the given name exists in the catalog.
func (c *Catalog) Exists(name string) bool {
	_, ok := c.Spec.CRDs[name]
	return ok
}

// Describe returns a human‑readable summary of a CRD.
func (c *Catalog) Describe(name string) (string, error) {
	crd, err := c.Get(name)
	if err != nil {
		return "", err
	}

	b := &strings.Builder{}
	fmt.Fprintf(b, "Name:        %s\n", crd.Name)
	fmt.Fprintf(b, "Group:       %s\n", crd.APITypes.Group)
	fmt.Fprintf(b, "Version:     %s\n", crd.APITypes.Version)
	fmt.Fprintf(b, "Kind:        %s\n", crd.APITypes.Kind)
	fmt.Fprintf(b, "Plural:      %s\n", crd.APITypes.Plural)
	fmt.Fprintf(b, "Namespaced:  %v\n", crd.Namespaced)
	if crd.IsNamespaced() {
		fmt.Fprintf(b, "Namespace:   %s\n", crd.Namespace)
	}
	fmt.Fprintf(b, "Workers:     %d\n", crd.SetWorkers(0))
	fmt.Fprintf(b, "Resync:      %s\n", crd.SetResync(0).String())
	fmt.Fprintf(b, "Enabled:     %v\n", crd.Enabled)

	deps := crd.DependsOn.Names()
	if len(deps) > 0 {
		fmt.Fprintf(b, "Dependencies: %v\n", strings.Join(deps, " "))
	} else {
		fmt.Fprint(b, "Dependencies: None")
	}

	fmt.Fprintf(b, "Description:\n%s\n", crd.Description)

	return b.String(), nil
}

// Explain returns a technical explanation of how Inrun handles this CRD.
func (c *Catalog) Explain(name string) (string, error) {
	crd, err := c.Get(name)
	if err != nil {
		return "", err
	}

	b := &strings.Builder{}
	fmt.Fprintf(b, "%s (%s/%s)\n", crd.APITypes.Kind, crd.APITypes.Group, crd.APITypes.Version)
	fmt.Fprintf(b, "----------------------------------------\n")
	fmt.Fprintf(b, "API Path:     %s\n", crd.APITypes.APIPath)
	fmt.Fprintf(b, "GVK:          %s\n", crd.GroupVersionKind.String())
	fmt.Fprint(b, "List Type:    runtime.Object")
	fmt.Fprint(b, "Object Type:  runtime.Object")
	if crd.DefaultReconcile() {
		fmt.Fprint(b, "operatorBox:   Default\n")
	} else {
		r := crd.Box().Reconcile
		if r != nil && r.Constructor != nil {
			fmt.Fprintf(b, "operatorBox:   %T\n", r.Constructor)
		} else {
			fmt.Fprint(b, "operatorBox:   Custom\n")
		}
	}
	fmt.Fprintf(b, "Informer:     LIST, WATCH\n")

	deps := crd.DependsOn.Names()
	if len(deps) > 0 {
		fmt.Fprintf(b, "Dependencies: %v\n", strings.Join(deps, " "))
	} else {
		fmt.Fprint(b, "Dependencies: None")
	}

	return b.String(), nil
}

// Graph returns a map of CRD name → dependency names.
func (c *Catalog) Graph() map[string][]string {
	graph := make(map[string][]string, c.Len())
	for name, crd := range c.enabledCRDs {
		graph[name] = crd.DependsOn.Names()
	}
	return graph
}

// Order returns CRDs in dependency‑safe order (topological sort).
func (c *Catalog) Order() []string {
	depGraph := NewDependencyGraph(c)
	return depGraph.ShutdownOrder()
}

// Controllers returns a list of CRDs that have reconcilers.
func (c *Catalog) Controllers() []string {
	var out []string
	for _, crd := range c.enabledCRDs {
		if crd.HasConstructor() {
			out = append(out, crd.Name)
		}
	}
	return out
}

// CRDNames returns the names of all enabled CRDs.
func (c *Catalog) CRDNames() []string {
	names := make([]string, 0, c.Len())
	for name := range c.enabledCRDs {
		names = append(names, name)
	}
	return names
}

// Depends returns true if crdName depends on target.
func (c *Catalog) Depends(crdName, target string) bool {
	crd, err := c.Get(crdName)
	if err != nil {
		return false
	}
	_, ok := crd.DependsOn[target]
	return ok
}

// Dependents returns all CRDs that depend on the given CRD.
func (c *Catalog) Dependents(name string) []string {
	var out []string
	for _, crd := range c.enabledCRDs {
		if _, ok := crd.DependsOn[name]; ok {
			out = append(out, crd.Name)
		}
	}
	return out
}

// Enabled returns only the enabled CRDs in the catalog.
func (c *Catalog) Enabled() map[string]types.CRDEntry {
	if c == nil {
		return nil
	}
	return c.enabledCRDs
}

// Get returns an enabled CRD by name.
func (c *Catalog) Get(name string) (*types.CRDEntry, error) {
	crd, ok := c.enabledCRDs[name]
	if !ok {
		return nil, fmt.Errorf("crd not found in catalog")
	}
	return &crd, nil
}

// ToUI returns a UI-friendly representation of the merged Catalog.
// This method extracts only the fields needed for display in the Console:
//   - API version and kind (always "Catalog" at runtime)
//   - Metadata (name, description, version, author, license)
//   - All merged CRD definitions
//
// Internal fields (Scheme, GroupVersionKind, etc.) are excluded because they
// have `yaml:"-" json:"-"` tags and won't be serialized to JSON.
//
// This method is used by the /catalog/raw endpoint to provide a clean,
// readable view of the Catalog that created the current operator.
func (c *Catalog) ToUI() *types.CatalogForUI {

	return &types.CatalogForUI{
		APIVersion: c.APIVersion,
		Kind:       config.CatalogKind(),
		Metadata:   c.metadata,
		Spec: types.CatalogSpecForUI{
			CRDs: c.Spec.CRDs,
		},
	}
}
