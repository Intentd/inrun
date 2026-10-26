package report

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/types"
)

// jSON renders the result as an indented JSON report.
func jSON(r *simulate.Result) ([]byte, error) {
	data, err := json.MarshalIndent(toReport(r), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding simulate report: %w", err)
	}
	return append(data, '\n'), nil
}

type caseReport struct {
	Field string `json:"field"`
	Error string `json:"error,omitempty"`
}

type resultReport struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type importReport struct {
	Path   string  `json:"path"`
	Result *report `json:"result,omitempty"`
	Error  string  `json:"error,omitempty"`
}

type report struct {
	Name    string         `json:"name"`
	Elapsed string         `json:"elapsed"`
	Steady  bool           `json:"steady"`
	Result  resultReport   `json:"result"`
	Cases   []caseReport   `json:"cases,omitempty"`
	Imports []importReport `json:"imports,omitempty"`
	// Comparisons — with --compare, the in-memory run against envtest.
	Comparisons []comparisonReport `json:"comparisons,omitempty"`
}

type comparisonReport struct {
	Name string `json:"name"`
	// Status — equivalent, different, or incomplete (a run failed).
	Status      string                   `json:"status"`
	Differences int                      `json:"differences"`
	Memory      runReport                `json:"memory"`
	Envtest     runReport                `json:"envtest"`
	Writes      simulate.WriteComparison `json:"writes"`
	Objects     []objectReport           `json:"objects,omitempty"`
}

type runReport struct {
	Steady   bool   `json:"steady"`
	SteadyAt int    `json:"steadyAt,omitempty"`
	Elapsed  string `json:"elapsed,omitempty"`
	Startup  string `json:"startup,omitempty"`
	Writes   int    `json:"writes"`
	Error    string `json:"error,omitempty"`
}

type objectReport struct {
	Ref         string               `json:"ref"`
	Matches     bool                 `json:"matches"`
	InMemory    bool                 `json:"inMemory"`
	InEnvtest   bool                 `json:"inEnvtest"`
	Differences []simulate.FieldDiff `json:"differences,omitempty"`
	Defaulted   []simulate.FieldDiff `json:"defaulted,omitempty"`
	Varies      []simulate.FieldDiff `json:"varies,omitempty"`
}

func toReport(r *simulate.Result) *report {
	out := &report{
		Name:    r.Name,
		Elapsed: r.Duration(),
		Steady:  r.Steady,
		Result: resultReport{
			Total:   r.Total(),
			Passed:  r.Passed(),
			Failed:  r.Failed(),
			Skipped: r.Skipped(),
		},
	}
	for _, c := range r.Assertions.Cases {
		item := caseReport{Field: c.Field}
		if c.Err != nil {
			item.Error = c.Err.Error()
		}
		out.Cases = append(out.Cases, item)
	}
	for _, c := range r.Comparisons {
		out.Comparisons = append(out.Comparisons, toComparisonReport(c))
	}
	for _, imp := range r.Imports {
		item := importReport{Path: imp.Path}
		if imp.Result != nil {
			item.Result = toReport(imp.Result)
		}
		if imp.Err != nil {
			item.Error = imp.Err.Error()
		}
		out.Imports = append(out.Imports, item)
	}
	return out
}

func toComparisonReport(c *simulate.Comparison) comparisonReport {
	out := comparisonReport{
		Name:        c.Name,
		Status:      comparisonStatus(c),
		Differences: c.Differences(),
		Memory:      toRunReport(c.Memory, c.MemoryErr),
		Envtest:     toRunReport(c.Envtest, c.EnvtestErr),
		Writes:      c.Writes,
	}
	for _, o := range c.Objects {
		out.Objects = append(out.Objects, objectReport{
			Ref: o.Ref(), Matches: o.Matches(), InMemory: o.InMemory, InEnvtest: o.InEnvtest,
			Differences: o.Differences, Defaulted: o.Defaulted, Varies: o.Varies,
		})
	}
	return out
}

func toRunReport(r *simulate.Result, err error) runReport {
	if err != nil || r == nil {
		out := runReport{}
		if err != nil {
			out.Error = err.Error()
		}
		return out
	}
	out := runReport{Steady: r.Steady, SteadyAt: r.SteadyAt, Elapsed: r.Elapsed.Round(time.Millisecond).String()}
	if r.Startup > 0 {
		out.Startup = r.Startup.Round(100 * time.Millisecond).String()
	}
	for _, op := range r.AllOps {
		if types.IsWriteVerb(op.Verb) {
			out.Writes++
		}
	}
	return out
}

// comparisonStatus names a comparison's outcome.
func comparisonStatus(c *simulate.Comparison) string {
	switch {
	case !c.Complete():
		return "incomplete"
	case c.Differences() > 0:
		return "different"
	default:
		return "equivalent"
	}
}
