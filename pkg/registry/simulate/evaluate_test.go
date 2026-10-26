package simulate

import (
	"errors"
	"testing"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestEvaluate_NoErrorsIsOneAssertion: however many cycles fail, noErrors
// fails once, under its own name, so the counts add up.
func TestEvaluate_NoErrorsIsOneAssertion(t *testing.T) {
	steady := true
	expect := &types.SimulateExpect{Steady: &steady, NoErrors: true}
	result := &Result{Steady: true, SteadyAt: 2}
	for i := 1; i <= 5; i++ {
		result.Cycles = append(result.Cycles, CycleResult{Cycle: i, Error: errors.New("connection refused")})
	}

	errs, err := Evaluate(result, expect, template.NewResolverWithFunc(types.KELFunctionRegistry{}), &unstructured.Unstructured{Object: map[string]any{}})
	require.Error(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, "noErrors", errs[0].Field)
	assert.Contains(t, errs[0].Message, "5 cycle(s) returned an error")
	assert.Equal(t, AssertionSummary{Status: result.Assertions.Status, Total: 2, Passed: 1, Failed: 1, Cases: result.Assertions.Cases}, result.Assertions)
}

// TestAssert_OpRuleNamespace: an op rule with a namespace matches only ops
// written in that namespace — each target of a cross-namespace copy.
func TestAssert_OpRuleNamespace(t *testing.T) {
	result := &Result{AllOps: []Op{
		{Cycle: 1, Verb: "apply", Resource: "secrets", Namespace: "team-alpha", Name: "db"},
	}}
	rule := func(ns string) types.SimulateOpRule {
		return types.SimulateOpRule{Cycle: 1, Verb: "apply", Resource: "secrets", Name: "db", Namespace: ns}
	}
	expect := &types.SimulateExpect{Ops: types.SimulateOpRules{rule("team-alpha"), rule("team-beta"), rule("")}}

	errs := Assert(result, expect, template.NewResolverWithFunc(types.KELFunctionRegistry{}), map[string]any{})
	require.Len(t, errs, 1, "only the team-beta rule fails")
	assert.Equal(t, "ops[1]", errs[0].Field)
	assert.Contains(t, errs[0].Message, "namespace=team-beta")
}
