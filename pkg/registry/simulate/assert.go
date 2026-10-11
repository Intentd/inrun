package simulate

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// AssertionError is one failed expectation from Assert.
type AssertionError struct {
	Field   string
	Message string
}

func (e AssertionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Assert checks a Result against a SimulateExpect.
// Returns nil when all expectations are satisfied.
func Assert(result *Result, expect *types.SimulateExpect) []AssertionError {
	if expect == nil {
		return nil
	}

	var errs []AssertionError

	if expect.Steady != nil && *expect.Steady && !result.Steady {
		errs = append(errs, AssertionError{
			Field:   "steady",
			Message: "expected steady state — reconciler did not stabilise within the cycle limit",
		})
	}

	if expect.SteadyAt != nil && result.SteadyAt > *expect.SteadyAt {
		errs = append(errs, AssertionError{
			Field:   "steadyAt",
			Message: fmt.Sprintf("expected steady at cycle ≤%d, reached at cycle %d", *expect.SteadyAt, result.SteadyAt),
		})
	}

	if expect.NoErrors {
		for _, c := range result.Cycles {
			if c.Error != nil {
				errs = append(errs, AssertionError{
					Field:   fmt.Sprintf("cycles[%d]", c.Cycle),
					Message: c.Error.Error(),
				})
			}
		}
	}

	for i, rule := range expect.Ops {
		n := opCount(result, rule)
		min := 1
		if rule.Count > 0 {
			min = rule.Count
		}
		if n < min {
			errs = append(errs, AssertionError{
				Field:   fmt.Sprintf("ops[%d]", i),
				Message: describeOpFailure(rule, n, min),
			})
		}
	}

	for i, rule := range expect.Absent {
		if n := opCount(result, rule); n > 0 {
			errs = append(errs, AssertionError{
				Field:   fmt.Sprintf("absent[%d]", i),
				Message: fmt.Sprintf("expected no op matching %s in cycle %d, found %d", describeRule(rule), rule.Cycle, n),
			})
		}
	}

	return errs
}

func describeRule(rule types.SimulateOpRule) string {
	s := fmt.Sprintf("verb=%s resource=%s", rule.Verb, rule.Resource)
	if rule.Name != "" {
		s += fmt.Sprintf(" name=%s", rule.Name)
	}
	return s
}

// ExpectForCRD returns the expect block for a named CRD.
// If expect.crds has an entry for crdName, that is returned.
// Otherwise the top-level expect (default) is returned.
func ExpectForCRD(expect *types.SimulateExpect, crdName string) *types.SimulateExpect {
	if expect == nil {
		return nil
	}
	if e, ok := expect.CRDs[crdName]; ok {
		return e
	}
	return expect
}

// opCount returns how many ops in result.AllOps match all non-empty fields
// of rule in the declared cycle.
func opCount(result *Result, rule types.SimulateOpRule) int {
	n := 0
	for _, op := range result.AllOps {
		if op.Cycle != rule.Cycle {
			continue
		}
		if rule.Verb != "" && op.Verb != rule.Verb {
			continue
		}
		if rule.Resource != "" && op.Resource != rule.Resource {
			continue
		}
		if rule.Name != "" && op.Name != rule.Name {
			continue
		}
		n++
	}
	return n
}

func describeOpFailure(rule types.SimulateOpRule, got, want int) string {
	desc := fmt.Sprintf("cycle=%d verb=%s resource=%s", rule.Cycle, rule.Verb, rule.Resource)
	if rule.Name != "" {
		desc += fmt.Sprintf(" name=%s", rule.Name)
	}
	if got == 0 {
		return fmt.Sprintf("no op matching %s", desc)
	}
	return fmt.Sprintf("wanted %d op(s) matching %s, got %d", want, desc, got)
}
