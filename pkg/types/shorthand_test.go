package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intentd/tools/condition"
)

// Every comparison in the table resolves the same way on a Condition and a
// ValidationRule, and each type maps every name.
func TestComparisons_ConditionAndRuleAgree(t *testing.T) {
	for _, cmp := range condition.Comparisons() {
		c, r := Condition{Condition: condition.Condition{Field: "spec.x"}}, ValidationRule{Field: "spec.x"}
		set(t, &c, &r, cmp.Name)

		op, v := ResolveConditionOp(c)
		assert.Equal(t, cmp.Op, op, cmp.Name)
		rop, rv := ResolveValidationOp(r)
		assert.Equal(t, cmp.Op, rop, cmp.Name)
		assert.Equal(t, v, rv, cmp.Name)
		assert.Equal(t, cmp.Name, RuleTypeLabel(r))
		assert.Equal(t, []string{cmp.Name}, c.Declared())
		assert.Equal(t, []string{cmp.Name}, r.Declared())
		assert.NoError(t, c.CheckComparison(), cmp.Name)
	}
}

func TestComparisons_RuleTypeOfMustBeAType(t *testing.T) {
	err := ValidationRule{Field: "spec.x", TypeOf: "list"}.CheckComparison()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "map, slice, string, number, bool, null")
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
