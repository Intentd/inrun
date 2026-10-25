//go:build !runtime && !gateway

package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/intentd/inrun/pkg/migrate"
)

// MigrateReport is the machine-readable form of a migrate, rollback, or clean
// run. The field names are a stable interface: consumers rely on them for CI
// checks, diffing, and audit logs. Change them only with a version bump.
//
// The report mirrors PrintMigrate/PrintRollback's vocabulary — "migrated",
// "drifted", "skipped", "failed" — so a script and a human reading the same
// run see the same buckets.
type MigrateReport struct {
	Mode       string       `json:"mode"` // "forward" | "rollback" | "clean"
	DryRun     bool         `json:"dryRun"`
	Blueprint  []string     `json:"blueprint"`
	Kind       string       `json:"kind,omitempty"`
	Name       string       `json:"name,omitempty"`
	Selector   string       `json:"selector,omitempty"`
	Namespaces []string     `json:"namespaces,omitempty"`
	Kinds      []KindReport `json:"kinds"`
	Totals     ReportTotals `json:"totals"`
}

// KindReport is the per-kind section of the report.
type KindReport struct {
	Kind  string     `json:"kind"`
	To    string     `json:"to,omitempty"`
	Scope *Scope     `json:"scope,omitempty"`
	CRs   []CRReport `json:"crs"`
}

// Scope is the selection a kind ran with: the blueprint's migrate.selection
// merged with the CLI flags.
type Scope struct {
	Namespaces []string `json:"namespaces,omitempty"`
	Name       string   `json:"name,omitempty"`
	Selector   string   `json:"selector,omitempty"`
}

// CRReport is one CR's outcome. Ref is "namespace/name" — the same form the
// human output uses. Fields, DriftedFields, and Error are omitted when empty
// so a clean run reads as small JSON.
type CRReport struct {
	Ref           string                 `json:"ref"`
	Fields        []string               `json:"fields,omitempty"`
	DriftedFields []string               `json:"driftedFields,omitempty"`
	AllDrifted    bool                   `json:"allDrifted,omitempty"`
	Drifted       bool                   `json:"drifted,omitempty"`
	Pruned        bool                   `json:"pruned,omitempty"`
	AfterSpec     map[string]interface{} `json:"afterSpec,omitempty"`
	Skipped       bool                   `json:"skipped,omitempty"`
	Excluded      bool                   `json:"excluded,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// ReportTotals aggregates the per-CR buckets across all kinds. A consumer can
// fail a pipeline on Totals.Failed > 0 or Totals.Drifted > 0 without walking
// the per-CR list.
type ReportTotals struct {
	CRs        int `json:"crs"`
	Migrated   int `json:"migrated"`
	Drifted    int `json:"drifted"`
	AllDrifted int `json:"allDrifted"`
	Skipped    int `json:"skipped"`
	Excluded   int `json:"excluded"`
	Pruned     int `json:"pruned"`
	Failed     int `json:"failed"`
}

// writeJSON writes the run's results as indented JSON to w. mode is one of
// "forward", "rollback", "clean" — the same strings that appear in the report.
func writeJSON(w io.Writer, mode string, opts Opts) error {
	report := buildReport(mode, opts)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// PrintMigrateJSON is the printFn-shaped entry point for JSON migrate output.
// Errors go to stderr so stdout stays a single valid JSON document.
func PrintMigrateJSON(opts Opts) {
	if err := writeJSON(os.Stdout, migrate.ModeForward.String(), opts); err != nil {
		fmt.Fprintf(os.Stderr, "marshal json: %v\n", err)
	}
}

// PrintRollbackJSON is the printFn-shaped entry point for JSON rollback output.
func PrintRollbackJSON(opts Opts) {
	if err := writeJSON(os.Stdout, migrate.ModeRollback.String(), opts); err != nil {
		fmt.Fprintf(os.Stderr, "marshal json: %v\n", err)
	}
}

// buildReport converts Opts into a MigrateReport, computing the same totals
// the human summary line shows.
func buildReport(mode string, opts Opts) MigrateReport {
	report := MigrateReport{
		Mode:       mode,
		DryRun:     opts.DryRun,
		Blueprint:  opts.Paths,
		Kind:       opts.Kind,
		Name:       opts.Name,
		Selector:   opts.Selector,
		Namespaces: opts.Namespaces,
	}
	for _, r := range opts.Results {
		if r == nil {
			continue
		}
		kind := KindReport{Kind: r.Kind, To: r.To}
		if !r.Selection.IsEmpty() {
			kind.Scope = &Scope{Namespaces: r.Selection.Namespaces, Name: r.Selection.Name, Selector: r.Selection.Selector}
		}
		for _, cr := range r.CRs {
			kind.CRs = append(kind.CRs, crReport(cr))
			report.Totals.CRs++
			switch {
			case cr.Err != nil:
				report.Totals.Failed++
			case cr.Excluded:
				report.Totals.Excluded++
			case cr.Skipped:
				report.Totals.Skipped++
			case cr.AllDrifted:
				report.Totals.AllDrifted++
			case cr.Drifted:
				report.Totals.Drifted++
			default:
				if len(cr.Fields) > 0 {
					report.Totals.Migrated++
				}
				if len(cr.DriftedFields) > 0 {
					report.Totals.Drifted++
				}
			}
			if cr.Err == nil && cr.Pruned {
				report.Totals.Pruned++
			}
		}
		report.Kinds = append(report.Kinds, kind)
	}
	return report
}

// crReport converts one migrate.CRMigrateResult to its JSON form. Error is stringified.
func crReport(cr migrate.CRMigrateResult) CRReport {
	out := CRReport{
		Ref:           cr.Ref(),
		Fields:        cr.Fields,
		DriftedFields: cr.DriftedFields,
		AllDrifted:    cr.AllDrifted,
		Drifted:       cr.Drifted,
		Pruned:        cr.Pruned,
		AfterSpec:     cr.AfterSpec,
		Skipped:       cr.Skipped,
		Excluded:      cr.Excluded,
	}
	if cr.Err != nil {
		out.Error = cr.Err.Error()
	}
	return out
}
