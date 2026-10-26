package report

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
)

// JSON renders the result as an indented JSON report.
func JSON(r *expect.Result) ([]byte, error) {
	data, err := json.MarshalIndent(toReport(r), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding E2E report: %w", err)
	}
	return append(data, '\n'), nil
}

func toReport(r *expect.Result) *report {
	var cases []caseReport
	if r.WithImports() {
		cases = make([]caseReport, 0, len(r.Cases))
	}
	out := &report{
		Name:       r.Name,
		SpecPath:   r.SpecPath,
		ClusterCtx: r.ClusterCtx,
		Cluster:    r.Cluster,
		Command:    r.Command,
		Result:     resultWithImports(r),
		Elapsed:    r.DurationWithImports(),
		Cases:      cases,
	}

	if !r.StartedAt.IsZero() {
		startedAt := r.StartedAt
		out.StartedAt = &startedAt
	}

	if r.StopReason != "" {
		out.Stop = &stopReport{
			Reason:  r.StopReason,
			After:   r.StoppedAfter,
			Ran:     r.Total(),
			Planned: r.Planned,
		}
	}

	for _, c := range r.Cases {
		item := caseReport{
			Name:    c.Name,
			Passed:  c.Passed,
			Skipped: c.Skipped,
			Elapsed: c.Elapsed.Round(time.Millisecond).String(),
		}
		if c.Err != nil {
			item.Error = c.Err.Error()
		}
		out.Cases = append(out.Cases, item)
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
