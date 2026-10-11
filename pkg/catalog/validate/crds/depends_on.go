package crds

import (
	"fmt"
)

func (c *checker) validateDependsOn() error {
	exists := make(map[string]bool, c.Cat.Len())
	for name := range c.Cat.EnabledCRDs() {
		exists[name] = true
	}

	for name, crd := range c.Cat.EnabledCRDs() {
		for dep := range crd.DependsOn {
			if dep == name {
				return fmt.Errorf("CRD '%s' cannot depend on itself", name)
			}
			if !exists[dep] {
				return fmt.Errorf("CRD '%s' depends on unknown or disabled CRD '%s'", name, dep)
			}
		}
	}

	return c.detectDependencyCycles()
}

func (c *checker) detectDependencyCycles() error {
	graph := make(map[string][]string, c.Cat.Len())
	for name, crd := range c.Cat.EnabledCRDs() {
		graph[name] = crd.DependsOn.Names()
	}

	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var dfs func(node string) error
	dfs = func(node string) error {
		if stack[node] {
			return fmt.Errorf("dependency cycle detected involving '%s'", node)
		}
		if visited[node] {
			return nil
		}

		visited[node] = true
		stack[node] = true

		for _, dep := range graph[node] {
			if err := dfs(dep); err != nil {
				return err
			}
		}

		stack[node] = false
		return nil
	}

	for name := range graph {
		if err := dfs(name); err != nil {
			return err
		}
	}

	return nil
}
