package types

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/util/structured"
)

// E2EAssertions checks a command's output, trimmed, with the comparisons
// conditions use and by the same names. Several may be set on one entry: each
// must hold. Embedded inline in every e2e command and kubectl check.
type E2EAssertions struct {
	// Equals — the output is exactly this text.
	Equals string `yaml:"equals,omitempty"`
	// NotEquals — the output is not this text.
	NotEquals string `yaml:"notEquals,omitempty"`
	// Prefix — the output starts with this text.
	Prefix string `yaml:"prefix,omitempty"`
	// NotPrefix — the output does not start with this text.
	NotPrefix string `yaml:"notPrefix,omitempty"`
	// Suffix — the output ends with this text.
	Suffix string `yaml:"suffix,omitempty"`
	// NotSuffix — the output does not end with this text.
	NotSuffix string `yaml:"notSuffix,omitempty"`
	// Contains — the output contains this text.
	Contains string `yaml:"contains,omitempty"`
	// NotContains — the output does not contain this text.
	NotContains string `yaml:"notContains,omitempty"`
	// Regex — the output matches this RE2 pattern.
	Regex string `yaml:"regex,omitempty"`
	// GreaterThan — the output, as a number, is above this one (strict).
	GreaterThan string `yaml:"greaterThan,omitempty"`
	// LessThan — the output, as a number, is below this one (strict).
	LessThan string `yaml:"lessThan,omitempty"`
	// Min — the output, as a number, is at least this.
	Min string `yaml:"min,omitempty"`
	// Max — the output, as a number, is at most this.
	Max string `yaml:"max,omitempty"`
	// Between — the output, as a number, is inside an inclusive "min,max".
	Between string `yaml:"between,omitempty"`
	// NotBetween — the output, as a number, is outside an inclusive "min,max".
	NotBetween string `yaml:"notBetween,omitempty"`
	// In — the output is one of a comma-separated list.
	In string `yaml:"in,omitempty"`
	// NotIn — the output is none of a comma-separated list.
	NotIn string `yaml:"notIn,omitempty"`
	// Exists — true: the output is non-empty.
	Exists *bool `yaml:"exists,omitempty"`
	// NotExists — true: the output is empty.
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

// JSONAssertion asserts on one value in a JSON document. Path selects the
// value (dot notation, e.g. ".totals.migrated" or ".kinds[0].crs[0].fields");
// the embedded E2EAssertions are evaluated against that value, exactly as
// they are against a command's stdout.
type JSONAssertion struct {
	// Path — dot path to the value to assert on, with optional [n] indexes.
	Path string `yaml:"path"`

	// Assertions on the extracted value: the same comparisons conditions use.
	E2EAssertions `yaml:",inline"`
}

type JSONAssertions []JSONAssertion

func (a JSONAssertions) Check(output string) error {
	if len(a) == 0 {
		return nil
	}
	var doc any
	if err := json.Unmarshal([]byte(output), &doc); err != nil {
		return fmt.Errorf("output is not valid JSON: %w", err)
	}
	for _, j := range a {
		v, _ := structured.LookupPath(doc, j.Path) // missing → nil → ""
		if err := j.E2EAssertions.Check(structured.RenderJSONValue(v)); err != nil {
			return fmt.Errorf("json %s: %w", j.Path, err)
		}
	}
	return nil
}

// Empty reports whether no JSON assertion is set.
func (a JSONAssertions) Empty() bool {
	return len(a) == 0
}

// Extractor selects one value from a command's output before assertions run.
// At most one field should be set; they are mutually exclusive modes.
type Extractor struct {
	// JSONPath selects one value from JSON output by dot path (in-process).
	JSONPath string `yaml:"jsonPath,omitempty"`
	// JQ selects one value from JSON output with a jq expression (requires format: json and jq).
	JQ string `yaml:"jq,omitempty"`
	// YQ selects one value from YAML output with a yq expression (requires format: yaml yq).
	YQ string `yaml:"yq,omitempty"`
}

// IsSet reports whether any extractor is configured.
func (e Extractor) IsSet() bool {
	return e.JSONPath != "" || e.JQ != "" || e.YQ != ""
}

// ExtractRequest is the input to ApplyExtract: the raw output plus the
// extractor to apply. WorkDir is where jq/yq run, if needed.
type ExtractRequest struct {
	WorkDir   string
	Output    string
	Extractor Extractor
}

// ApplyExtract returns the value to assert on. JSONPath is tried first
// (in-process, no binary); JQ and YQ then shell out. With no extractor set,
// output is returned unchanged.
func ApplyExtract(ctx context.Context, req ExtractRequest) (string, error) {
	switch {
	case req.Extractor.JSONPath != "":
		var doc any
		if err := json.Unmarshal([]byte(req.Output), &doc); err != nil {
			return "", fmt.Errorf("jsonPath %q: output is not valid JSON: %w", req.Extractor.JSONPath, err)
		}
		v, _ := structured.LookupPath(doc, req.Extractor.JSONPath)
		return structured.RenderJSONValue(v), nil
	case req.Extractor.JQ != "":
		jq := req.Extractor.JQ
		if !strings.HasPrefix(jq, ".") {
			jq = "." + jq
		}
		cmd := exec.CommandContext(ctx, "jq", "-r", jq)
		if req.WorkDir != "" {
			cmd.Dir = req.WorkDir
		}
		cmd.Stdin = strings.NewReader(req.Output)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("jq %q: %w\ninput: %s", jq, err, req.Output)
		}
		return strings.TrimSpace(string(out)), nil
	case req.Extractor.YQ != "":
		yq := req.Extractor.YQ
		cmd := exec.CommandContext(ctx, "yq", "e", yq, "-")
		if req.WorkDir != "" {
			cmd.Dir = req.WorkDir
		}
		cmd.Stdin = strings.NewReader(req.Output)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("yq %q: %w\ninput: %s", yq, err, req.Output)
		}
		return strings.TrimSpace(string(out)), nil
	default:
		return req.Output, nil
	}
}
