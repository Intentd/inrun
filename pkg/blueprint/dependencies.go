// pkg/blueprint/dependencies.go
package blueprint

import (
	"fmt"
	"sort"
	"sync"

	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"

	"github.com/intentd/inrun/pkg/types"
)

// DependencyGraph represents the CRD dependency DAG for a Blueprint.
// Each CRD is a node, and edges represent "A must start before B" relationships.
//
// The graph is used to:
//   - compute deterministic startup order (topological sort)
//   - compute deterministic shutdown order (reverse topological sort)
//   - validate dependency correctness (no cycles, no missing CRDs)
//   - expose dependency metadata to the runtime
//
// The graph is immutable after construction.
type DependencyGraph struct {
	nodes        map[string]*Node    // CRD name → node
	edges        map[string][]string // dependency → list of dependents
	blueprint    *Blueprint          // owning blueprint
	startupOrder []string            // cached topological order
	once         sync.Once           // ensures StartupOrder is computed once
}

// Node represents a single CRD in the dependency graph.
type Node struct {
	Name      string
	CRD       types.CRDEntry
	InDegree  int // number of CRDs this CRD depends on
	OutDegree int // number of CRDs that depend on this CRD
}

// NewDependencyGraph constructs the dependency DAG for all enabled CRDs.
// It validates that all declared dependencies exist and builds the adjacency lists.
func NewDependencyGraph(blueprint *Blueprint) *DependencyGraph {
	crds := blueprint.enabledCRDs
	g := &DependencyGraph{
		nodes:     make(map[string]*Node),
		edges:     make(map[string][]string),
		blueprint: blueprint,
	}

	// Create nodes for each CRD
	for name, crd := range crds {
		g.nodes[name] = &Node{
			Name: name,
			CRD:  crd,
		}
	}

	// Create directed edges: dep → dependent
	// Example: if B depends on A, then edge A → B
	for name, crd := range crds {
		for dep := range crd.DependsOn {
			if _, exists := g.nodes[dep]; !exists {
				term.Exit(fmt.Errorf("dependency %s not found for %s", dep, name))
			}
			g.edges[dep] = append(g.edges[dep], name)
			g.nodes[name].InDegree++
			g.nodes[dep].OutDegree++
		}
	}

	return g
}

// StartupOrder returns a deterministic topological ordering of CRDs.
// CRDs with no dependencies appear first; dependents appear after their prerequisites.
//
// This order is used by the runtime to start CRDs in the correct sequence.
//
// The result is cached and computed only once.
func (g *DependencyGraph) StartupOrder() []string {
	g.once.Do(func() {
		// Copy indegree so we don't mutate original graph
		indegree := make(map[string]int, len(g.nodes))
		for name, node := range g.nodes {
			indegree[name] = node.InDegree
		}

		// Deterministic ordering: sort all node names
		var names []string
		for name := range g.nodes {
			names = append(names, name)
		}
		sort.Strings(names)

		// Initialize queue with all nodes that have no dependencies
		var queue []string
		for _, name := range names {
			if indegree[name] == 0 {
				queue = append(queue, name)
			}
		}

		var order []string

		// Kahn's algorithm (deterministic variant)
		for len(queue) > 0 {
			// Always pick the first (queue is kept sorted)
			current := queue[0]
			queue = queue[1:]

			order = append(order, current)

			// Process dependents in sorted order
			neighbors := append([]string{}, g.edges[current]...)
			sort.Strings(neighbors)

			for _, neighbor := range neighbors {
				indegree[neighbor]--
				if indegree[neighbor] == 0 {
					queue = append(queue, neighbor)
				}
			}

			// Keep queue sorted for determinism
			sort.Strings(queue)
		}

		if len(order) != len(g.nodes) {
			term.Exit(fmt.Errorf("circular dependency detected"))
		}

		g.startupOrder = order
	})

	return g.startupOrder
}

// ShutdownOrder returns the reverse of the startup order.
// This ensures CRDs are stopped only after all dependents have been drained.
func (g *DependencyGraph) ShutdownOrder() []string {
	// Reverse it
	return structured.Reversed(g.StartupOrder())
}

// GetNode returns the node for a CRD name.
func (g *DependencyGraph) GetNode(name string) *Node {
	return g.nodes[name]
}

// GetNodes returns all nodes in the graph.
func (g *DependencyGraph) GetNodes() map[string]*Node {
	return g.nodes
}

// GetEdges returns the adjacency list: dependency → dependents.
func (g *DependencyGraph) GetEdges() map[string][]string {
	return g.edges
}

// GetInDegree returns how many CRDs this CRD depends on.
func (g *DependencyGraph) GetInDegree(name string) int {
	return g.nodes[name].InDegree
}

// GetOutDegree returns how many CRDs depend on this CRD.
func (g *DependencyGraph) GetOutDegree(name string) int {
	return g.nodes[name].OutDegree
}

// GetDependents returns all CRDs that depend on the given CRD.
func (g *DependencyGraph) GetDependents(name string) []string {
	var dependents []string
	for dep, deps := range g.edges {
		for _, d := range deps {
			if d == name {
				dependents = append(dependents, dep)
			}
		}
	}
	return dependents
}

// GetDependencies returns all CRDs that the given CRD depends on.
func (g *DependencyGraph) GetDependencies(name string) []string {
	return g.edges[name]
}

// Validate performs basic sanity checks on the graph.
func (g *DependencyGraph) Validate() error {
	for name, node := range g.nodes {
		if node.InDegree < 0 {
			return fmt.Errorf("negative in-degree for %s", name)
		}
		if node.OutDegree < 0 {
			return fmt.Errorf("negative out-degree for %s", name)
		}
	}
	return nil
}
