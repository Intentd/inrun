package simulate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// AssertionResult is one failed expectation from Assert.
type AssertionResult struct {
	Field   string
	Message string
	Skipped bool
}

type AssertionResults []AssertionResult

func (a AssertionResults) SetSkipped(field string) AssertionResults {
	return append(a, AssertionResult{
		Field:   field,
		Skipped: true,
	})
}

func (e AssertionResult) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Assert checks a Result against a SimulateExpect.
// Returns nil when all expectations are satisfied.
func Assert(result *Result, expect *types.SimulateExpect, resolver *template.Resolver, data map[string]interface{}) AssertionResults {
	if expect == nil {
		return nil
	}

	var results AssertionResults

	if !types.EvaluateConditions(data, expect.When, expect.Or, resolver.TemplateEvaluator()) {
		return results.SetSkipped("expect")
	}

	var errs AssertionResults

	if expect.Steady != nil && *expect.Steady && !result.Steady {
		errs = append(errs, AssertionResult{
			Field:   "steady",
			Message: "expected steady state — reconciler did not stabilise within the cycle limit",
		})
	}

	if expect.SteadyAt != nil && result.SteadyAt > *expect.SteadyAt {
		errs = append(errs, AssertionResult{
			Field:   "steadyAt",
			Message: fmt.Sprintf("expected steady at cycle ≤%d, reached at cycle %d", *expect.SteadyAt, result.SteadyAt),
		})
	}

	// noErrors is one assertion: it fails once, naming every cycle whose
	// reconcile returned an error.
	if expect.NoErrors {
		var failed []string
		for _, c := range result.Cycles {
			if c.Error != nil {
				failed = append(failed, fmt.Sprintf("cycle %d: %v", c.Cycle, c.Error))
			}
		}
		if len(failed) > 0 {
			errs = append(errs, AssertionResult{
				Field:   "noErrors",
				Message: fmt.Sprintf("%d cycle(s) returned an error — %s", len(failed), strings.Join(failed, "; ")),
			})
		}
	}

	for i, rule := range expect.Ops {
		field := fmt.Sprintf("ops[%d]", i)

		if !types.EvaluateConditions(data, rule.When, rule.Or, resolver.TemplateEvaluator()) {
			results = results.SetSkipped(field)
			continue
		}

		if rule.Verb != "" || rule.Resource != "" {
			n := opCount(result, rule)
			min := 1
			if rule.Count > 0 {
				min = rule.Count
			}
			if n < min {
				errs = append(errs, AssertionResult{
					Field:   field,
					Message: describeOpFailure(rule, n, min),
				})
			}
		}

		cycleErr := cycleErrorText(result, rule.Cycle)

		if inline := (types.Assertions{TextAssertion: rule.TextAssertion}); !inline.Empty() {
			if err := inline.Check(cycleErr); err != nil {
				errs = append(errs, AssertionResult{
					Field:   field + ".assert",
					Message: err.Error(),
				})
			}
		}

		if rule.Assert != nil {
			if err := rule.Assert.Check(cycleErr); err != nil {
				errs = append(errs, AssertionResult{
					Field:   field + ".assert",
					Message: err.Error(),
				})
			}
		}
	}

	for i, rule := range expect.Absent {
		field := fmt.Sprintf("absent[%d]", i)

		if !types.EvaluateConditions(data, rule.When, rule.Or, resolver.TemplateEvaluator()) {
			results = results.SetSkipped(field)
			continue
		}

		if n := opCount(result, rule); n > 0 {
			errs = append(errs, AssertionResult{
				Field:   field,
				Message: fmt.Sprintf("expected no op matching %s in cycle %d, found %d", describeRule(rule), rule.Cycle, n),
			})
		}
	}

	return append(results, errs...)
}

func describeRule(rule types.SimulateOpRule) string {
	s := fmt.Sprintf("verb=%s resource=%s", rule.Verb, rule.Resource)
	if rule.Name != "" {
		s += fmt.Sprintf(" name=%s", rule.Name)
	}
	if rule.Namespace != "" {
		s += fmt.Sprintf(" namespace=%s", rule.Namespace)
	}
	return s
}

// CheckExpectCRDs reports expect.crds keys that name no CRD in crdNames: their
// expectations would never run.
func CheckExpectCRDs(expect *types.SimulateExpect, crdNames []string) error {
	if expect == nil || len(expect.CRDs) == 0 {
		return nil
	}
	var unknown []string
	for name := range expect.CRDs {
		if !slices.Contains(crdNames, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	known := slices.Clone(crdNames)
	slices.Sort(known)
	return fmt.Errorf("expect.crds: no CRD %s in the Blueprint (its CRDs: %s)",
		strings.Join(unknown, ", "), strings.Join(known, ", "))
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
		if rule.Namespace != "" && op.Namespace != rule.Namespace {
			continue
		}
		n++
	}
	return n
}

// cycleErrorText returns the error string for the given cycle number, or ""
// when the cycle succeeded or was not found.
func cycleErrorText(result *Result, cycle int) string {
	for _, c := range result.Cycles {
		if c.Cycle == cycle {
			if c.Error != nil {
				return c.Error.Error()
			}
			return ""
		}
	}
	return ""
}

func describeOpFailure(rule types.SimulateOpRule, got, want int) string {
	desc := fmt.Sprintf("cycle=%d verb=%s resource=%s", rule.Cycle, rule.Verb, rule.Resource)
	if rule.Name != "" {
		desc += fmt.Sprintf(" name=%s", rule.Name)
	}
	if rule.Namespace != "" {
		desc += fmt.Sprintf(" namespace=%s", rule.Namespace)
	}
	if got == 0 {
		return fmt.Sprintf("no op matching %s", desc)
	}
	return fmt.Sprintf("wanted %d op(s) matching %s, got %d", want, desc, got)
}
