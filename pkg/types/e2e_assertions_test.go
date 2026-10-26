package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssertions_Check(t *testing.T) {
	yes := true
	cases := []struct {
		name   string
		output string
		a      Assertions
		pass   bool
	}{
		{"contains", "hello world", Assertions{TextAssertion: TextAssertion{Contains: "world"}}, true},
		{"contains missing", "hello world", Assertions{TextAssertion: TextAssertion{Contains: "missing"}}, false},
		{"notContains", "hello world", Assertions{TextAssertion: TextAssertion{NotContains: "missing"}}, true},
		{"notContains present", "hello world", Assertions{TextAssertion: TextAssertion{NotContains: "world"}}, false},
		{"equals trims", "  exact  \n", Assertions{TextAssertion: TextAssertion{Equals: "exact"}}, true},
		{"notEquals", "actual", Assertions{TextAssertion: TextAssertion{NotEquals: "expected"}}, true},
		{"exists on empty", "", Assertions{TextAssertion: TextAssertion{Exists: &yes}}, false},
		{"notExists on output", "something", Assertions{TextAssertion: TextAssertion{NotExists: &yes}}, false},
		{"in", "b", Assertions{TextAssertion: TextAssertion{In: "a,b,c"}}, true},
		{"in missing", "z", Assertions{TextAssertion: TextAssertion{In: "a,b,c"}}, false},
		{"notIn", "z", Assertions{TextAssertion: TextAssertion{NotIn: "a,b,c"}}, true},
		{"notIn present", "b", Assertions{TextAssertion: TextAssertion{NotIn: "a,b,c"}}, false},
		{"greaterThan is strict", "5", Assertions{TextAssertion: TextAssertion{GreaterThan: "5"}}, false},
		{"greaterThan", "6", Assertions{TextAssertion: TextAssertion{GreaterThan: "5"}}, true},
		{"lessThan is strict", "5", Assertions{TextAssertion: TextAssertion{LessThan: "5"}}, false},
		{"lessThan", "4", Assertions{TextAssertion: TextAssertion{LessThan: "5"}}, true},
		{"not a number", "not-a-number", Assertions{TextAssertion: TextAssertion{GreaterThan: "5"}}, false},
		{"min is inclusive", "5", Assertions{TextAssertion: TextAssertion{Min: "5"}}, true},
		{"min", "4", Assertions{TextAssertion: TextAssertion{Min: "5"}}, false},
		{"max is inclusive", "5", Assertions{TextAssertion: TextAssertion{Max: "5"}}, true},
		{"max", "6", Assertions{TextAssertion: TextAssertion{Max: "5"}}, false},
		{"between inclusive", "10", Assertions{TextAssertion: TextAssertion{Between: "1,10"}}, true},
		{"between outside", "11", Assertions{TextAssertion: TextAssertion{Between: "1,10"}}, false},
		{"notBetween outside", "11", Assertions{TextAssertion: TextAssertion{NotBetween: "1,10"}}, true},
		{"notBetween inside", "5", Assertions{TextAssertion: TextAssertion{NotBetween: "1,10"}}, false},
		{"regex", "app-prod-01", Assertions{TextAssertion: TextAssertion{Regex: `^app-\w+-\d+$`}}, true},
		{"regex no match", "APP", Assertions{TextAssertion: TextAssertion{Regex: `^app-\w+-\d+$`}}, false},
		{"prefix", "app-prod", Assertions{TextAssertion: TextAssertion{Prefix: "app-"}}, true},
		{"all must hold", "5", Assertions{TextAssertion: TextAssertion{Contains: "5", GreaterThan: "10"}}, false},
		{"none set", "anything", Assertions{}, true},
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

func TestAssertions_FailureNamesTheAssertion(t *testing.T) {
	err := Assertions{TextAssertion: TextAssertion{Min: "10"}}.Check("3")
	assert.EqualError(t, err, `output "3" fails min "10"`)
}

func TestAssertionNames_FromTheTable(t *testing.T) {
	names := AssertionNames()
	assert.Equal(t, []string{"exists", "notExists"}, names[:2])
	assert.Contains(t, names, "min")
	assert.NotContains(t, names, "unique")
	assert.NotContains(t, names, "typeOf")
	assert.True(t, Assertions{}.Empty())
}

// One step checks its output inline, then text, then json: all must hold, and
// a failure says which form and entry failed.
func TestAssertions_TextAndJSON(t *testing.T) {
	out := `{"status":"ok","count":2}`
	all := Assertions{
		TextAssertion: TextAssertion{Contains: `"status":"ok"`},
		Text:          TextAssertions{{NotContains: "error"}, {Regex: `"count":[0-9]+`}},
		JSON:          JSONAssertions{{Path: ".count", TextAssertion: TextAssertion{GreaterThan: "1"}}},
	}
	assert.NoError(t, all.Check(out))
	assert.True(t, all.HasText())
	assert.True(t, all.HasJSON())

	for name, tc := range map[string]struct {
		a    Assertions
		want string
	}{
		"inline":   {Assertions{TextAssertion: TextAssertion{Contains: "missing"}}, `fails contains "missing"`},
		"text":     {Assertions{Text: TextAssertions{{Contains: "status"}, {Contains: "missing"}}}, "text[1]:"},
		"json":     {Assertions{JSON: JSONAssertions{{Path: ".count", TextAssertion: TextAssertion{Equals: "3"}}}}, "json .count:"},
		"not json": {Assertions{JSON: JSONAssertions{{Path: ".x"}}}, "not valid JSON"},
	} {
		err := tc.a.Check(map[bool]string{true: "plain text", false: out}[name == "not json"])
		if assert.Error(t, err, name) {
			assert.Contains(t, err.Error(), tc.want, name)
		}
	}

	inline := Assertions{TextAssertion: TextAssertion{Contains: "plain"}}
	assert.NoError(t, inline.Check("plain text"), "an inline-only step never parses its output as JSON")
	assert.False(t, inline.HasText())
	assert.False(t, inline.HasJSON())
}
