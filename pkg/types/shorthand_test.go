package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every comparison in the table resolves the same way on a Condition and a
// ValidationRule, and each type maps every name.
func TestComparisons_ConditionAndRuleAgree(t *testing.T) {
	for _, cmp := range append(append([]comparison{}, flags...), comparisons...) {
		c, r := Condition{Field: "spec.x"}, ValidationRule{Field: "spec.x"}
		set(t, &c, &r, cmp.name)

		op, v := ResolveConditionOp(c)
		assert.Equal(t, cmp.op, op, cmp.name)
		rop, rv := ResolveValidationOp(r)
		assert.Equal(t, cmp.op, rop, cmp.name)
		assert.Equal(t, v, rv, cmp.name)
		assert.Equal(t, cmp.name, RuleTypeLabel(r))
		assert.Equal(t, []string{cmp.name}, c.Declared())
		assert.Equal(t, []string{cmp.name}, r.Declared())
		assert.NoError(t, c.CheckComparison(), cmp.name)
	}
}

func TestComparisons_NoneMeansExists(t *testing.T) {
	op, _ := ResolveConditionOp(Condition{Field: "spec.x"})
	assert.Equal(t, ConditionExists, op)
}

func TestComparisons_MoreThanOneRejected(t *testing.T) {
	err := Condition{Field: "spec.x", Min: "1", Max: "3"}.CheckComparison()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "declares min and max")
}

func TestComparisons_TypeOfMustBeAType(t *testing.T) {
	assert.NoError(t, Condition{Field: "spec.x", TypeOf: "slice"}.CheckComparison())
	err := ValidationRule{Field: "spec.x", TypeOf: "list"}.CheckComparison()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "map, slice, string, number, bool, null")
}

func TestComparisonNames_FlagsThenValues(t *testing.T) {
	names := ComparisonNames()
	assert.Equal(t, "exists", names[0])
	assert.Contains(t, names, "typeOf")
	assert.NotContains(t, names, "greaterThanOrEqual")
}

func set(t *testing.T, c *Condition, r *ValidationRule, name string) {
	t.Helper()
	yes := true
	switch name {
	case "exists":
		c.Exists, r.Exists = &yes, &yes
		return
	case "notExists":
		c.NotExists, r.NotExists = &yes, &yes
		return
	case "unique":
		c.Unique, r.Unique = &yes, &yes
		return
	case "typeOf":
		c.TypeOf, r.TypeOf = FieldTypeString, FieldTypeString
		return
	}
	fields := map[string][2]*string{
		"equals": {&c.Equals, &r.Equals}, "notEquals": {&c.NotEquals, &r.NotEquals},
		"prefix": {&c.Prefix, &r.Prefix}, "notPrefix": {&c.NotPrefix, &r.NotPrefix},
		"suffix": {&c.Suffix, &r.Suffix}, "notSuffix": {&c.NotSuffix, &r.NotSuffix},
		"contains": {&c.Contains, &r.Contains}, "notContains": {&c.NotContains, &r.NotContains},
		"regex":       {&c.Regex, &r.Regex},
		"greaterThan": {&c.GreaterThan, &r.GreaterThan}, "lessThan": {&c.LessThan, &r.LessThan},
		"min": {&c.Min, &r.Min}, "max": {&c.Max, &r.Max},
		"between": {&c.Between, &r.Between}, "notBetween": {&c.NotBetween, &r.NotBetween},
		"in": {&c.In, &r.In}, "notIn": {&c.NotIn, &r.NotIn},
	}
	f, ok := fields[name]
	if !ok {
		t.Fatalf("no field for comparison %q", name)
	}
	*f[0], *f[1] = "v", "v"
}
