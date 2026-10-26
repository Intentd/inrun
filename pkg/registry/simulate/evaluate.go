package simulate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Evaluate checks result against expect, records the outcome in
// result.Assertions, and returns each assertion's outcome and the error that
// fails the run. Without expectations a cycle error fails the run; with
// them, noErrors decides, so a spec can assert a CR that is denied. data is
// the CR, for the expectations' when: and or: conditions.
func Evaluate(result *Result, expect *types.SimulateExpect, resolver *template.Resolver, cr *unstructured.Unstructured) (AssertionResults, error) {
	if expect == nil {
		if result.CycleErrors() > 0 {
			result.Assertions.Status = registry.StatusFailed
			return nil, fmt.Errorf("simulation completed with cycle errors")
		}
		result.Assertions.Status = registry.StatusNoAssertion
		return nil, nil
	}
	errs := Assert(result, expect, resolver, cr.Object)
	return errs, summarise(result, expect, errs)
}

// CycleErrors counts the cycles whose reconcile returned an error.
func (r *Result) CycleErrors() int {
	n := 0
	for _, c := range r.Cycles {
		if c.Error != nil {
			n++
		}
	}
	return n
}

// summarise counts expect's assertions and errs' outcomes into
// result.Assertions and sets its status.
func summarise(result *Result, expect *types.SimulateExpect, errs AssertionResults) error {
	result.Assertions.Total = len(expect.Ops) + len(expect.Absent) +
		boolInt(expect.Steady != nil) +
		boolInt(expect.SteadyAt != nil) +
		boolInt(expect.NoErrors)

	for _, e := range errs {
		if e.Skipped {
			result.Assertions.Skipped++
			result.Assertions.Cases = append(result.Assertions.Cases, CaseResult{Field: e.Field, Skipped: true})
			continue
		}
		result.Assertions.Failed++
		result.Assertions.Cases = append(result.Assertions.Cases, CaseResult{Field: e.Field, Err: e})
	}

	result.Assertions.Passed = result.Assertions.Total - result.Assertions.Failed - result.Assertions.Skipped

	if result.Assertions.Failed > 0 {
		result.Assertions.Status = registry.StatusFailed
		return fmt.Errorf("assertions failed (%d/%d passed)", result.Assertions.Passed, result.Assertions.Total)
	}
	result.Assertions.Status = registry.StatusPassed
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
