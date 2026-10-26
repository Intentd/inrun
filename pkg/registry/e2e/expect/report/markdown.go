package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
)

// Markdown renders the result as a GitHub-flavoured markdown summary block.
func Markdown(r *expect.Result) string {
	var b strings.Builder
	writeMarkdown(r, &b, 2)
	return b.String()
}

func writeMarkdown(r *expect.Result, b *strings.Builder, level int) {
	heading := strings.Repeat("#", level)

	var passed, failed, skipped []expect.CaseResult
	for _, c := range r.Cases {
		switch {
		case c.Skipped:
			skipped = append(skipped, c)
		case c.Passed:
			passed = append(passed, c)
		default:
			failed = append(failed, c)
		}
	}

	fmt.Fprintf(b, "%s E2E Results: %s\n\n", heading, r.Name)

	b.WriteString("| Property | Value |\n|---|---|\n")
	fmt.Fprintf(b, "| Spec | %s |\n", r.SpecPath)

	if r.ClusterCtx != "" {
		fmt.Fprintf(b, "| Cluster context | %s |\n", r.ClusterCtx)
	}
	if r.Cluster != "" {
		fmt.Fprintf(b, "| Cluster | %s |\n", r.Cluster)
	}
	if !r.StartedAt.IsZero() {
		fmt.Fprintf(b, "| Started | %s |\n", r.StartedAt.Format(time.RFC3339))
	}

	if r.WithImports() {
		fmt.Fprintf(b, "| Duration | %s |\n\n", r.DurationWithImports())
	} else {
		fmt.Fprintf(b, "| Duration | %s |\n\n", r.Duration())
	}

	// Results for this suite and all imported tests.
	result := resultWithImports(r)

	b.WriteString("| Result | |\n|---|---|\n")
	fmt.Fprintf(b, "| Total | %d |\n", result.Total)
	fmt.Fprintf(b, "| ✅ Passed | %d |\n", result.Passed)
	fmt.Fprintf(b, "| ❌ Failed | %d |\n", result.Failed)
	fmt.Fprintf(b, "| ⏭ Skipped | %d |\n\n", result.Skipped)

	if r.StopReason != "" {
		failureStr := "failure"
		if r.StoppedAfter != 1 {
			failureStr = "failures"
		}

		fmt.Fprintf(b,
			"> ⛔ **Stopped** after %d %s (%s) — %d of %d expectations run\n\n",
			r.StoppedAfter,
			failureStr,
			r.StopReason,
			r.Total(),
			r.Planned,
		)
	}

	if r.Command != "" {
		b.WriteString("**Command:**\n\n```bash\n")
		b.WriteString(r.Command)
		b.WriteString("\n```\n\n")
	}

	writeCases := func(title string, cases []expect.CaseResult, withError bool) {
		if len(cases) == 0 {
			return
		}

		fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level+1), title)

		if withError {
			b.WriteString("| Test | Time | Error |\n|---|---|---|\n")

			for _, c := range cases {
				msg := ""
				if c.Err != nil {
					msg = strings.ReplaceAll(
						strings.TrimSpace(c.Err.Error()), "\n", " ",
					)
				}

				fmt.Fprintf(b, "| ❌ %s | %s | %s |\n",
					c.Name,
					c.Elapsed.Round(time.Millisecond),
					msg,
				)
			}
		} else {
			b.WriteString("| Test | Time |\n|---|---|\n")

			for _, c := range cases {
				mark := "✅"
				if c.Skipped {
					mark = "⏭"
				}

				fmt.Fprintf(b, "| %s %s | %s |\n",
					mark,
					c.Name,
					c.Elapsed.Round(time.Millisecond),
				)
			}
		}

		b.WriteString("\n")
	}

	writeCases("Failed", failed, true)
	writeCases("Passed", passed, false)
	writeCases("Skipped", skipped, false)

	if r.WithImports() {
		fmt.Fprintf(b, "%s Imported E2E Suites\n\n",
			strings.Repeat("#", level+1))

		for _, imp := range r.Imports {
			if imp.Result != nil {
				writeMarkdown(imp.Result, b, level+2)
			} else {
				fmt.Fprintf(b, "%s %s\n\n",
					strings.Repeat("#", level+2),
					imp.Name(),
				)
			}

			if imp.Err != nil {
				b.WriteString("❌ **Import failed:** ")
				b.WriteString(strings.ReplaceAll(imp.Err.Error(), "\n", " "))
				b.WriteString("\n\n")
			}
		}
	}

	fmt.Fprintf(b, "**%s**\n\n", r.Summary())
}
