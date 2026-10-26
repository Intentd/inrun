package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestE2EAssertions_Check(t *testing.T) {
	yes := true
	cases := []struct {
		name   string
		output string
		a      E2EAssertions
		pass   bool
	}{
		{"contains", "hello world", E2EAssertions{Contains: "world"}, true},
		{"contains missing", "hello world", E2EAssertions{Contains: "missing"}, false},
		{"notContains", "hello world", E2EAssertions{NotContains: "missing"}, true},
		{"notContains present", "hello world", E2EAssertions{NotContains: "world"}, false},
		{"equals trims", "  exact  \n", E2EAssertions{Equals: "exact"}, true},
		{"notEquals", "actual", E2EAssertions{NotEquals: "expected"}, true},
		{"exists on empty", "", E2EAssertions{Exists: &yes}, false},
		{"notExists on output", "something", E2EAssertions{NotExists: &yes}, false},
		{"in", "b", E2EAssertions{In: "a,b,c"}, true},
		{"in missing", "z", E2EAssertions{In: "a,b,c"}, false},
		{"notIn", "z", E2EAssertions{NotIn: "a,b,c"}, true},
		{"notIn present", "b", E2EAssertions{NotIn: "a,b,c"}, false},
		{"greaterThan is strict", "5", E2EAssertions{GreaterThan: "5"}, false},
		{"greaterThan", "6", E2EAssertions{GreaterThan: "5"}, true},
		{"lessThan is strict", "5", E2EAssertions{LessThan: "5"}, false},
		{"lessThan", "4", E2EAssertions{LessThan: "5"}, true},
		{"not a number", "not-a-number", E2EAssertions{GreaterThan: "5"}, false},
		{"min is inclusive", "5", E2EAssertions{Min: "5"}, true},
		{"min", "4", E2EAssertions{Min: "5"}, false},
		{"max is inclusive", "5", E2EAssertions{Max: "5"}, true},
		{"max", "6", E2EAssertions{Max: "5"}, false},
		{"between inclusive", "10", E2EAssertions{Between: "1,10"}, true},
		{"between outside", "11", E2EAssertions{Between: "1,10"}, false},
		{"notBetween outside", "11", E2EAssertions{NotBetween: "1,10"}, true},
		{"notBetween inside", "5", E2EAssertions{NotBetween: "1,10"}, false},
		{"regex", "app-prod-01", E2EAssertions{Regex: `^app-\w+-\d+$`}, true},
		{"regex no match", "APP", E2EAssertions{Regex: `^app-\w+-\d+$`}, false},
		{"prefix", "app-prod", E2EAssertions{Prefix: "app-"}, true},
		{"all must hold", "5", E2EAssertions{Contains: "5", GreaterThan: "10"}, false},
		{"none set", "anything", E2EAssertions{}, true},
	}
	for _, c := range cases {
		err := c.a.Check(c.output)
		if c.pass {
			assert.NoError(t, err, c.name)
		} else {
			assert.Error(t, err, c.name)
		}
	}
}

func TestE2EAssertions_FailureNamesTheAssertion(t *testing.T) {
	err := E2EAssertions{Min: "10"}.Check("3")
	assert.EqualError(t, err, `output "3" fails min "10"`)
}

func TestE2EAssertionNames_FromTheTable(t *testing.T) {
	names := E2EAssertionNames()
	assert.Equal(t, []string{"exists", "notExists"}, names[:2])
	assert.Contains(t, names, "min")
	assert.NotContains(t, names, "unique")
	assert.NotContains(t, names, "typeOf")
	assert.True(t, E2EAssertions{}.Empty())
}
