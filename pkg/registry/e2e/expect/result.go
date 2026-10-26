package expect

import (
	"fmt"
	"strings"
	"time"
)

// Result holds the outcome of a complete E2E run.
// It is returned by Run and consumed by inrun push to embed
// verification metadata into OCI annotations.
type Result struct {
	Name       string
	SpecPath   string
	ClusterCtx string
	Cluster    string
	StartedAt  time.Time
	Cases      []CaseResult
	Elapsed    time.Duration
	Command    string
	// StopReason is non-empty when the run stopped before all expectations ran.
	// Values: "max-failures" (the --max-failures cap was hit) or "step" (an
	// expectation declared onFailure.stop). Empty means the run completed.
	StopReason string
	// StoppedAfter is the number of failures observed when the run stopped.
	// Zero when StopReason is empty.
	StoppedAfter int
	Planned      int // the number of expectations declared in the suite
	// Imports contains the results of imported E2E suites.
	Imports ImportResults `json:"-"`
}

// CaseResult holds the outcome of a single expectation.
type CaseResult struct {
	Name    string
	Passed  bool
	Skipped bool
	Elapsed time.Duration
	Err     error
}

// ImportResult holds the outcome of one imported E2E file in a suite run.
type ImportResult struct {
	// Path is the import path as declared in imports:
	// (e.g. "./01-basic/e2e.yaml").
	Path string
	// Result is the structured outcome. Nil when the import failed to load or
	// the runner itself errored before producing any expectations.
	Result *Result
	// Err is non-nil when the import failed (load error, Inrun not ready,
	// or one or more expectations failed).
	Err error
}

// AllPassed returns true when every non-skipped expectation passed.
func (r *Result) AllPassed() bool {
	for _, c := range r.Cases {
		if !c.Passed && !c.Skipped {
			return false
		}
	}
	return true
}

// Passed returns the number of passing expectations.
func (r *Result) Passed() int {
	n := 0
	for _, c := range r.Cases {
		if c.Passed {
			n++
		}
	}
	return n
}

// Skipped returns the number of skipped expectations.
func (r *Result) Skipped() int {
	n := 0
	for _, c := range r.Cases {
		if c.Skipped {
			n++
		}
	}
	return n
}

// Failed returns the number of failed expectations.
func (r *Result) Failed() int {
	n := 0
	for _, c := range r.Cases {
		if !c.Passed && !c.Skipped {
			n++
		}
	}
	return n
}

// Total returns the total number of expectations.
func (r *Result) Total() int {
	if r == nil {
		return 0
	}
	return len(r.Cases)
}

// Duration returns the total elapsed time as a human-readable string (e.g. "45s").
func (r *Result) Duration() string {
	if r == nil {
		return "0s"
	}
	return r.Elapsed.Round(time.Second).String()
}

// WithImports returns the total elapsed time across this suite and all imports.
func (r *Result) WithImportsDuration() time.Duration {
	if r == nil {
		return 0
	}

	return r.Elapsed + r.Imports.Duration()
}

// DurationWithImports returns the total elapsed time as a human-readable string.
func (r *Result) DurationWithImports() string {
	if !r.WithImports() {
		return r.Duration()
	}
	return r.WithImportsDuration().Round(time.Second).String()
}

// WithImports returns true when the suite has imported E2E suites.
func (r *Result) WithImports() bool {
	return r != nil && len(r.Imports) > 0
}

// Summary returns a one-line result string, e.g. "3 of 4 passed, 1 skipped (45s)".
func (r *Result) Summary() string {
	if r == nil {
		return "0s"
	}
	total := len(r.Cases)
	s := fmt.Sprintf("%d of %d passed", r.Passed(), total)
	if n := r.Skipped(); n > 0 {
		s += fmt.Sprintf(", %d skipped", n)
	}
	return s + fmt.Sprintf(" (%s)", r.Duration())
}

// Name returns the display name — the E2E metadata.name when available,
// otherwise the import path.
func (i ImportResult) Name() string {
	if i.Result != nil && i.Result.Name != "" {
		return i.Result.Name
	}
	return i.Path
}

// Passed reports whether this import succeeded.
func (i ImportResult) Passed() bool {
	return i.Err == nil
}

type ImportResults []ImportResult

// Total returns the total number of expectations across all imported suites.
func (r ImportResults) Total() int {
	total := 0
	for _, ir := range r {
		if ir.Result != nil {
			total += ir.Result.Total()
			total += ir.Result.Imports.Total()
		}
	}
	return total
}

// Passed returns the number of passing expectations across all imported suites.
func (r ImportResults) Passed() int {
	passed := 0
	for _, ir := range r {
		if ir.Result != nil {
			passed += ir.Result.Passed()
			passed += ir.Result.Imports.Passed()
		}
	}
	return passed
}

// Failed returns the number of failed expectations across all imported suites.
func (r ImportResults) Failed() int {
	failed := 0
	for _, ir := range r {
		if ir.Result != nil {
			failed += ir.Result.Failed()
			failed += ir.Result.Imports.Failed()
		}
	}
	return failed
}

// Skipped returns the number of skipped expectations across all imported suites.
func (r ImportResults) Skipped() int {
	skipped := 0
	for _, ir := range r {
		if ir.Result != nil {
			skipped += ir.Result.Skipped()
			skipped += ir.Result.Imports.Skipped()
		}
	}
	return skipped
}

// Duration returns the total elapsed time across all imported suites,
// including nested imports.
func (r ImportResults) Duration() time.Duration {
	var duration time.Duration

	for _, ir := range r {
		if ir.Result != nil {
			duration += ir.Result.Elapsed
			duration += ir.Result.Imports.Duration()
		}
	}

	return duration
}

// PrintImportSummary prints the suite results table and returns a non-nil
// error when any import failed.
func (r ImportResults) PrintImportSummary(suiteName string) error {
	passed := 0
	for _, ir := range r {
		if ir.Passed() {
			passed++
		}
	}

	var total time.Duration
	for _, ir := range r {
		if ir.Result != nil {
			total += ir.Result.Elapsed
		}
	}

	fmt.Printf("\n─── Suite Results: %s ───\n\n", suiteName)
	for _, ir := range r {
		icon := "✓"
		if !ir.Passed() {
			icon = "✗"
		}

		score := "—"
		elapsed := ""
		if ir.Result != nil {
			score = fmt.Sprintf("%d / %d", ir.Result.Passed(), ir.Result.Total())
			elapsed = ir.Result.Duration()
		}

		errSuffix := ""
		if ir.Err != nil {
			msg := ir.Err.Error()
			// Trim the "import ./path: " prefix if present so the table stays narrow.
			if idx := strings.Index(msg, ": "); idx >= 0 {
				msg = msg[idx+2:]
			}
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			errSuffix = "  → " + msg
		}

		fmt.Printf("  %s  %-44s  %-6s  %-6s%s\n", icon, ir.Name(), score, elapsed, errSuffix)
	}

	elapsed := total.Round(time.Second).String()
	fmt.Printf("\n  %d of %d imports passed  (%s)\n", passed, len(r), elapsed)

	if passed < len(r) {
		return fmt.Errorf("%d of %d imports failed", len(r)-passed, len(r))
	}
	return nil
}
