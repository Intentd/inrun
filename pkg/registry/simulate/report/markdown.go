package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/registry/simulate"
)

// markdown renders the result as a GitHub-flavoured markdown summary block.
func markdown(r *simulate.Result) string {
	var b strings.Builder
	writeMarkdown(r, &b, 2)
	return b.String()
}

func writeMarkdown(r *simulate.Result, b *strings.Builder, level int) {
	heading := strings.Repeat("#", level)

	fmt.Fprintf(b, "%s Simulate Results: %s\n\n", heading, r.Name)

	b.WriteString("| Property | Value |\n|---|---|\n")
	fmt.Fprintf(b, "| Duration | %s |\n", r.Elapsed.Round(time.Millisecond))
	fmt.Fprintf(b, "| Steady | %v |\n\n", r.Steady)

	b.WriteString("| Result | |\n|---|---|\n")
	fmt.Fprintf(b, "| Total | %d |\n", r.Total())
	fmt.Fprintf(b, "| ✅ Passed | %d |\n", r.Passed())
	fmt.Fprintf(b, "| ❌ Failed | %d |\n", r.Failed())
	fmt.Fprintf(b, "| ⏭️ Skipped | %d |\n\n", r.Skipped())

	var passed, failed, skipped []simulate.CaseResult
	for _, c := range r.Assertions.Cases {
		switch {
		case c.Skipped:
			skipped = append(skipped, c)
		case c.Err != nil:
			failed = append(failed, c)
		default:
			passed = append(passed, c)
		}
	}

	writeCases := func(title string, cases []simulate.CaseResult, status string, withError bool) {
		if len(cases) == 0 {
			return
		}

		fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level+1), title)

		if withError {
			b.WriteString("| Field | Error |\n|---|---|\n")
			for _, c := range cases {
				msg := ""
				if c.Err != nil {
					msg = strings.ReplaceAll(strings.TrimSpace(c.Err.Error()), "\n", " ")
				}
				fmt.Fprintf(b, "| %s %s | %s |\n", status, c.Field, msg)
			}
		} else {
			b.WriteString("| Field |\n|---|\n")
			for _, c := range cases {
				fmt.Fprintf(b, "| %s %s |\n", status, c.Field)
			}
		}
		b.WriteString("\n")
	}

	writeCases("Failed", failed, "❌", true)
	writeCases("Passed", passed, "✅", false)
	writeCases("Skipped", skipped, "⏭️", false)

	for _, c := range r.Comparisons {
		writeComparison(c, b, level+1)
	}

	if r.WithImports() {
		fmt.Fprintf(b, "%s Imported Suites\n\n", strings.Repeat("#", level+1))
		for _, imp := range r.Imports {
			if imp.Result != nil {
				writeMarkdown(imp.Result, b, level+2)
			} else {
				fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level+2), imp.Path)
			}
			if imp.Err != nil {
				b.WriteString("❌ **Import failed:** ")
				b.WriteString(strings.ReplaceAll(imp.Err.Error(), "\n", " "))
				b.WriteString("\n\n")
			}
		}
	}

	fmt.Fprintf(b, "**%s**\n\n", summary(r))
}

func summary(r *simulate.Result) string {
	switch {
	case r.Failed() > 0:
		return fmt.Sprintf("%d/%d assertion(s) passed · %d failed · %d skipped",
			r.Passed(), r.Total(), r.Failed(), r.Skipped())
	case r.Total() == 0:
		return "No assertions"
	case r.Skipped() > 0:
		return fmt.Sprintf("%d passed · %d skipped",
			r.Passed(), r.Skipped())
	default:
		return fmt.Sprintf("All %d assertion(s) passed", r.Total())
	}
}

// writeComparison renders one in-memory vs envtest comparison: each run's
// outcome, then every write and object difference with its field.
func writeComparison(c *simulate.Comparison, b *strings.Builder, level int) {
	fmt.Fprintf(b, "%s Comparison: %s (%s)\n\n", strings.Repeat("#", level), c.Name, comparisonStatus(c))

	b.WriteString("| Run | Steady | Reconcile time | Error |\n|---|---|---|---|\n")
	for _, run := range []struct {
		name string
		r    *simulate.Result
		err  error
	}{{"In-memory", c.Memory, c.MemoryErr}, {"Envtest", c.Envtest, c.EnvtestErr}} {
		if run.err != nil || run.r == nil {
			msg := ""
			if run.err != nil {
				msg = strings.ReplaceAll(run.err.Error(), "\n", " ")
			}
			fmt.Fprintf(b, "| %s | – | – | %s |\n", run.name, msg)
			continue
		}
		steady := "no"
		if run.r.Steady {
			steady = fmt.Sprintf("cycle %d", run.r.SteadyAt)
		}
		fmt.Fprintf(b, "| %s | %s | %s | |\n", run.name, steady, run.r.Elapsed.Round(time.Millisecond))
	}
	b.WriteString("\n")
	if !c.Complete() {
		return
	}

	fmt.Fprintf(b, "%d write(s) match.\n\n", c.Writes.Matched)
	if len(c.Writes.Differences) > 0 {
		b.WriteString("| Write | In-memory | Envtest |\n|---|---|---|\n")
		for _, d := range c.Writes.Differences {
			fmt.Fprintf(b, "| ❌ %s %s | %d× | %d× |\n", d.Verb, d.Ref(), d.Memory, d.Envtest)
		}
		b.WriteString("\n")
	}

	b.WriteString("| Object | Result |\n|---|---|\n")
	for _, o := range c.Objects {
		switch {
		case !o.InEnvtest:
			fmt.Fprintf(b, "| ❌ %s | only in-memory |\n", o.Ref())
		case !o.InMemory:
			fmt.Fprintf(b, "| ❌ %s | only in envtest |\n", o.Ref())
		case len(o.Differences) > 0:
			fmt.Fprintf(b, "| ❌ %s | %d field(s) differ |\n", o.Ref(), len(o.Differences))
		default:
			fmt.Fprintf(b, "| ✅ %s | matches (%d field(s) defaulted by the API server, %d random by design) |\n", o.Ref(), len(o.Defaulted), len(o.Varies))
		}
	}
	b.WriteString("\n")

	for _, o := range c.Objects {
		if len(o.Differences) == 0 {
			continue
		}
		fmt.Fprintf(b, "**%s**\n\n| Field | In-memory | Envtest |\n|---|---|---|\n", o.Ref())
		for _, d := range o.Differences {
			fmt.Fprintf(b, "| `%s` | %s | %s |\n", d.Path, mdValue(d.Memory), mdValue(d.Envtest))
		}
		b.WriteString("\n")
	}
}

func mdValue(v any) string {
	if v == nil {
		return "_absent_"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return "`" + strings.ReplaceAll(string(raw), "|", "\\|") + "`"
}
