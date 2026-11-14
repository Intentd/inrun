package types

import "github.com/intentd/tools/condition"

// ComparisonNames returns every comparison a condition or validation rule can
// declare, flags first, for messages that list them.
func ComparisonNames() []string { return condition.ComparisonNames() }

// Declared returns the names of the comparisons the rule sets; a valid rule
// sets at most one.
func (r ValidationRule) Declared() []string {
	return condition.Names(condition.Declared(r.flagValues(), r.values()))
}

func (r ValidationRule) flagValues() map[string]*bool {
	return map[string]*bool{"exists": r.Exists, "notExists": r.NotExists, "unique": r.Unique}
}

func (r ValidationRule) values() map[string]string {
	return map[string]string{
		"equals": r.Equals, "notEquals": r.NotEquals,
		"prefix": r.Prefix, "notPrefix": r.NotPrefix,
		"suffix": r.Suffix, "notSuffix": r.NotSuffix,
		"contains": r.Contains, "notContains": r.NotContains,
		"regex":       r.Regex,
		"greaterThan": r.GreaterThan, "lessThan": r.LessThan,
		"min": r.Min, "max": r.Max,
		"between": r.Between, "notBetween": r.NotBetween,
		"in": r.In, "notIn": r.NotIn,
		"typeOf": string(r.TypeOf),
	}
}

// CheckComparison reports a rule that declares more than one comparison, or
// a typeOf that is not a type.
func (r ValidationRule) CheckComparison() error {
	return condition.CheckComparisons(condition.Declared(r.flagValues(), r.values()), r.TypeOf)
}
