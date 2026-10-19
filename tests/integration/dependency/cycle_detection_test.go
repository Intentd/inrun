//go:build integration

// tests/integration/dependency/cycle_detection_test.go
package dependency_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate"
	"github.com/intentd/inrun/pkg/types"
)

func TestCycleDetection_TwoNodeCycle(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"a": {Name: "a", DependsOn: types.DependsOnMap{"b": {}}},
		"b": {Name: "b", DependsOn: types.DependsOnMap{"a": {}}},
	})
	if err := validate.DetectCycles(bp); err == nil {
		t.Error("expected cycle error for a ↔ b")
	}
}

func TestCycleDetection_ThreeNodeCycle(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"a": {Name: "a", DependsOn: types.DependsOnMap{"c": {}}},
		"b": {Name: "b", DependsOn: types.DependsOnMap{"a": {}}},
		"c": {Name: "c", DependsOn: types.DependsOnMap{"b": {}}},
	})
	if err := validate.DetectCycles(bp); err == nil {
		t.Error("expected cycle error for a → c → b → a")
	}
}

func TestCycleDetection_SelfLoop(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"a": {Name: "a", DependsOn: types.DependsOnMap{"a": {}}},
	})
	if err := validate.DetectCycles(bp); err == nil {
		t.Error("expected cycle error for self-loop")
	}
}

func TestCycleDetection_AcyclicGraph_NoError(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{
		"db":    {Name: "db"},
		"cache": {Name: "cache"},
		"app":   {Name: "app", DependsOn: types.DependsOnMap{"db": {}, "cache": {}}},
	})
	if err := validate.DetectCycles(bp); err != nil {
		t.Errorf("acyclic graph must not produce cycle error: %v", err)
	}
}

func TestCycleDetection_EmptyGraph_NoError(t *testing.T) {
	bp := blueprint.NewBlueprintForTest(map[string]types.CRDEntry{})
	if err := validate.DetectCycles(bp); err != nil {
		t.Errorf("empty graph must not produce cycle error: %v", err)
	}
}
