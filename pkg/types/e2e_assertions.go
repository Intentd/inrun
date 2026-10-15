package types

import (
	"fmt"
	"strings"
)

// E2EAssertions checks a command's output, trimmed, with the comparisons
// conditions use and by the same names. Several may be set on one entry: each
// must hold. Embedded inline in every e2e command and kubectl check.
type E2EAssertions struct {
	Equals      string `yaml:"equals,omitempty"`
	NotEquals   string `yaml:"notEquals,omitempty"`
	Prefix      string `yaml:"prefix,omitempty"`
	NotPrefix   string `yaml:"notPrefix,omitempty"`
	Suffix      string `yaml:"suffix,omitempty"`
	NotSuffix   string `yaml:"notSuffix,omitempty"`
	Contains    string `yaml:"contains,omitempty"`
	NotContains string `yaml:"notContains,omitempty"`
	// Regex is an RE2 pattern.
	Regex string `yaml:"regex,omitempty"`
	// GreaterThan and LessThan are strict; Min and Max inclusive. The output
	// is parsed as a number.
	GreaterThan string `yaml:"greaterThan,omitempty"`
	LessThan    string `yaml:"lessThan,omitempty"`
	Min         string `yaml:"min,omitempty"`
	Max         string `yaml:"max,omitempty"`
	// Between and NotBetween take an inclusive "min,max".
	Between    string `yaml:"between,omitempty"`
	NotBetween string `yaml:"notBetween,omitempty"`
	// In and NotIn take a comma-separated list.
	In    string `yaml:"in,omitempty"`
	NotIn string `yaml:"notIn,omitempty"`
	// Exists asserts the output is non-empty; NotExists that it is empty.
	Exists    *bool `yaml:"exists,omitempty"`
	NotExists *bool `yaml:"notExists,omitempty"`
}

func (a E2EAssertions) flagValues() map[string]*bool {
	return map[string]*bool{"exists": a.Exists, "notExists": a.NotExists}
}

func (a E2EAssertions) values() map[string]string {
	return map[string]string{
		"equals": a.Equals, "notEquals": a.NotEquals,
		"prefix": a.Prefix, "notPrefix": a.NotPrefix,
		"suffix": a.Suffix, "notSuffix": a.NotSuffix,
		"contains": a.Contains, "notContains": a.NotContains,
		"regex":       a.Regex,
		"greaterThan": a.GreaterThan, "lessThan": a.LessThan,
		"min": a.Min, "max": a.Max,
		"between": a.Between, "notBetween": a.NotBetween,
		"in": a.In, "notIn": a.NotIn,
	}
}

// Empty reports whether no assertion is set.
func (a E2EAssertions) Empty() bool {
	return len(declared(a.flagValues(), a.values())) == 0
}

// Check returns an error naming the first assertion output fails, nil when
// all hold. Evaluated by the same operators as conditions.
func (a E2EAssertions) Check(output string) error {
	trimmed := strings.TrimSpace(output)
	data := map[string]interface{}{"output": trimmed}
	values := a.values()
	for _, c := range declared(a.flagValues(), values) {
		want := values[c.name]
		if applyOperator(c.op, trimmed, want, data, Condition{Field: "output"}) {
			continue
		}
		if want == "" {
			return fmt.Errorf("output %q fails %s", trimmed, c.name)
		}
		return fmt.Errorf("output %q fails %s %q", trimmed, c.name, want)
	}
	return nil
}

// E2EAssertionNames returns the assertions an e2e check can declare, in the
// comparison table's order.
func E2EAssertionNames() []string {
	var a E2EAssertions
	flagVals, values := a.flagValues(), a.values()
	var out []string
	for _, name := range ComparisonNames() {
		_, isFlag := flagVals[name]
		_, isValue := values[name]
		if isFlag || isValue {
			out = append(out, name)
		}
	}
	return out
}
