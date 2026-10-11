package crds

import (
	"testing"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/types"
)

func crdMapForCycle(crds ...types.CRDEntry) map[string]types.CRDEntry {
	m := make(map[string]types.CRDEntry, len(crds))
	for _, c := range crds {
		m[c.Name] = c
	}
	return m
}

func depEntry(name string, deps ...string) types.CRDEntry {
	e := types.CRDEntry{Name: name}
	if len(deps) > 0 {
		e.DependsOn = make(types.DependsOnMap, len(deps))
		for _, d := range deps {
			e.DependsOn[d] = types.DependsOnCondition{Condition: "started"}
		}
	}
	return e
}

func TestDetectCycles_TwoNodeCycle(t *testing.T) {
	cat := catalog.NewCatalogForTest(crdMapForCycle(depEntry("a", "b"), depEntry("b", "a")))
	if err := DetectCycles(cat); err == nil {
		t.Error("expected cycle detection error for a ↔ b cycle")
	}
}

func TestDetectCycles_ThreeNodeCycle(t *testing.T) {
	cat := catalog.NewCatalogForTest(crdMapForCycle(depEntry("a", "c"), depEntry("b", "a"), depEntry("c", "b")))
	if err := DetectCycles(cat); err == nil {
		t.Error("expected cycle detection error for a → c → b → a cycle")
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	cat := catalog.NewCatalogForTest(crdMapForCycle(depEntry("a"), depEntry("b", "a"), depEntry("c", "b")))
	if err := DetectCycles(cat); err != nil {
		t.Errorf("expected no error for acyclic graph, got %v", err)
	}
}
