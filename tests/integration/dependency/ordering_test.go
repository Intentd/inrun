//go:build integration

// tests/integration/dependency/ordering_test.go
package dependency_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/types"
)

func mkCRD(name string, dependsOn ...string) types.CRDEntry {
	e := types.CRDEntry{Name: name}
	if len(dependsOn) > 0 {
		e.DependsOn = make(types.DependsOnMap, len(dependsOn))
		for _, d := range dependsOn {
			e.DependsOn[d] = types.DependsOnCondition{}
		}
	}
	return e
}

func assertBefore(t *testing.T, order []string, first, second string) {
	t.Helper()
	fi, si := -1, -1
	for i, v := range order {
		if v == first {
			fi = i
		}
		if v == second {
			si = i
		}
	}
	if fi < 0 {
		t.Errorf("%q not in order %v", first, order)
		return
	}
	if si < 0 {
		t.Errorf("%q not in order %v", second, order)
		return
	}
	if fi >= si {
		t.Errorf("expected %q before %q in %v", first, second, order)
	}
}

func mkMap(crds ...types.CRDEntry) map[string]types.CRDEntry {
	m := make(map[string]types.CRDEntry, len(crds))
	for _, c := range crds {
		m[c.Name] = c
	}
	return m
}

func TestStartupOrder_LinearChain(t *testing.T) {
	cat := catalog.NewCatalogForTest(mkMap(
		mkCRD("app", "cache"),
		mkCRD("cache", "database"),
		mkCRD("database"),
	))
	order := catalog.NewDependencyGraph(cat).StartupOrder()
	assertBefore(t, order, "database", "cache")
	assertBefore(t, order, "cache", "app")
}

func TestStartupOrder_IndependentCRDs_AllPresent(t *testing.T) {
	cat := catalog.NewCatalogForTest(mkMap(
		mkCRD("alpha"), mkCRD("beta"), mkCRD("gamma"),
	))
	if n := len(catalog.NewDependencyGraph(cat).StartupOrder()); n != 3 {
		t.Errorf("expected 3 CRDs in order, got %d", n)
	}
}

func TestStartupOrder_DiamondDependency(t *testing.T) {
	//        base
	//       /    \
	//    left   right
	//       \    /
	//        app
	cat := catalog.NewCatalogForTest(mkMap(
		mkCRD("app", "left", "right"),
		mkCRD("left", "base"),
		mkCRD("right", "base"),
		mkCRD("base"),
	))
	order := catalog.NewDependencyGraph(cat).StartupOrder()
	assertBefore(t, order, "base", "left")
	assertBefore(t, order, "base", "right")
	assertBefore(t, order, "left", "app")
	assertBefore(t, order, "right", "app")
}

func TestShutdownOrder_ReversesStartup(t *testing.T) {
	cat := catalog.NewCatalogForTest(mkMap(
		mkCRD("app", "db"),
		mkCRD("db"),
	))
	g := catalog.NewDependencyGraph(cat)
	startup, shutdown := g.StartupOrder(), g.ShutdownOrder()
	for i := range startup {
		if startup[i] != shutdown[len(shutdown)-1-i] {
			t.Errorf("startup/shutdown mismatch at position %d", i)
		}
	}
}

func TestStartupOrder_SingleCRD(t *testing.T) {
	cat := catalog.NewCatalogForTest(mkMap(mkCRD("solo")))
	order := catalog.NewDependencyGraph(cat).StartupOrder()
	if len(order) != 1 || order[0] != "solo" {
		t.Errorf("expected [solo], got %v", order)
	}
}
