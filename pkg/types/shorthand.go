package types

import (
	"fmt"
	"slices"
	"strings"
)

// comparison is one way Condition and ValidationRule compare a field
// (equals: x, min: "3", unique: true, …): its YAML name and the operator the
// evaluators run for it.
type comparison struct {
	name string
	op   ConditionOperator
}

// flags are the comparisons set with true: they take no value.
var flags = []comparison{
	{"exists", ConditionExists},
	{"notExists", ConditionNotExists},
	{"unique", ConditionUnique},
}

// comparisons take a value. min and max are inclusive bounds; greaterThan and
// lessThan are strict.
var comparisons = []comparison{
	{"equals", ConditionEquals},
	{"notEquals", ConditionNotEquals},
	{"prefix", ConditionPrefix},
	{"notPrefix", ConditionNotPrefix},
	{"suffix", ConditionSuffix},
	{"notSuffix", ConditionNotSuffix},
	{"contains", ConditionContains},
	{"notContains", ConditionNotContains},
	{"regex", ConditionRegex},
	{"greaterThan", ConditionGt},
	{"lessThan", ConditionLt},
	{"min", ConditionGte},
	{"max", ConditionLte},
	{"between", ConditionBetween},
	{"notBetween", ConditionNotBetween},
	{"in", ConditionIn},
	{"notIn", ConditionNotIn},
	{"typeOf", ConditionTypeOf},
}

// FieldType is a value's type as typeOf names it: the names the typeOf
// template function returns.
type FieldType string

const (
	FieldTypeMap    FieldType = "map"
	FieldTypeSlice  FieldType = "slice"
	FieldTypeString FieldType = "string"
	FieldTypeNumber FieldType = "number"
	FieldTypeBool   FieldType = "bool"
	FieldTypeNull   FieldType = "null"
)

// FieldTypes is every FieldType, for validation and messages.
var FieldTypes = []FieldType{FieldTypeMap, FieldTypeSlice, FieldTypeString, FieldTypeNumber, FieldTypeBool, FieldTypeNull}

// IsValid reports whether t is one of FieldTypes.
func (t FieldType) IsValid() bool { return slices.Contains(FieldTypes, t) }

func fieldTypeNames() string {
	out := make([]string, len(FieldTypes))
	for i, t := range FieldTypes {
		out[i] = string(t)
	}
	return strings.Join(out, ", ")
}

// ComparisonNames returns every comparison a condition or validation rule can
// declare, flags first, for messages that list them.
func ComparisonNames() []string {
	out := make([]string, 0, len(flags)+len(comparisons))
	for _, c := range append(append([]comparison{}, flags...), comparisons...) {
		out = append(out, c.name)
	}
	return out
}

// declared returns the comparisons set among flagVals and values, in table
// order: a flag counts when true, a comparison when non-empty.
func declared(flagVals map[string]*bool, values map[string]string) []comparison {
	var out []comparison
	for _, f := range flags {
		if b := flagVals[f.name]; b != nil && *b {
			out = append(out, f)
		}
	}
	for _, c := range comparisons {
		if values[c.name] != "" {
			out = append(out, c)
		}
	}
	return out
}

// resolve returns the operator and value of the one comparison declared;
// exists when none is. Validation rejects more than one, so the first wins
// only for an unvalidated rule.
func resolve(flagVals map[string]*bool, values map[string]string) (string, ConditionOperator, string) {
	if set := declared(flagVals, values); len(set) > 0 {
		return set[0].name, set[0].op, values[set[0].name]
	}
	return "exists", ConditionExists, ""
}

func names(set []comparison) []string {
	out := make([]string, len(set))
	for i, c := range set {
		out[i] = c.name
	}
	return out
}

// Declared returns the names of the comparisons the condition sets; a valid
// condition sets at most one.
func (c Condition) Declared() []string { return names(declared(c.flagValues(), c.values())) }

// Declared returns the names of the comparisons the rule sets; a valid rule
// sets at most one.
func (r ValidationRule) Declared() []string { return names(declared(r.flagValues(), r.values())) }

func (c Condition) flagValues() map[string]*bool {
	return map[string]*bool{"exists": c.Exists, "notExists": c.NotExists, "unique": c.Unique}
}

func (c Condition) values() map[string]string {
	return map[string]string{
		"equals": c.Equals, "notEquals": c.NotEquals,
		"prefix": c.Prefix, "notPrefix": c.NotPrefix,
		"suffix": c.Suffix, "notSuffix": c.NotSuffix,
		"contains": c.Contains, "notContains": c.NotContains,
		"regex":       c.Regex,
		"greaterThan": c.GreaterThan, "lessThan": c.LessThan,
		"min": c.Min, "max": c.Max,
		"between": c.Between, "notBetween": c.NotBetween,
		"in": c.In, "notIn": c.NotIn,
		"typeOf": string(c.TypeOf),
	}
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

// CheckComparison reports a condition that declares more than one comparison,
// or a typeOf that is not a type.
func (c Condition) CheckComparison() error {
	return checkComparison(declared(c.flagValues(), c.values()), c.TypeOf)
}

// CheckComparison reports a rule that declares more than one comparison, or
// a typeOf that is not a type.
func (r ValidationRule) CheckComparison() error {
	return checkComparison(declared(r.flagValues(), r.values()), r.TypeOf)
}

func checkComparison(set []comparison, typeOf FieldType) error {
	if len(set) > 1 {
		return fmt.Errorf("declares %s: a condition compares one way. Split it into several conditions "+
			"under when (all must hold) or or (one must hold)", strings.Join(names(set), " and "))
	}
	if typeOf != "" && !typeOf.IsValid() {
		return fmt.Errorf("typeOf %q is not a type: use one of %s", typeOf, fieldTypeNames())
	}
	return nil
}
