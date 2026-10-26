//go:build !runtime && !gateway

package output

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/migrate"
	sigsyaml "sigs.k8s.io/yaml"
)

func PrintMigrate(opts Opts) {
	fmt.Println()
	op := "migrate"
	if opts.Clean {
		op = "migrate clean"
	}
	label := cmdutil.Bold("⟳") + "  inrun " + op
	if opts.DryRun {
		label += "  " + cmdutil.Yellow("[dry-run]")
	}
	fmt.Println(label)
	fmt.Printf("  %s %s\n", cmdutil.Gray("blueprint:"), cmdutil.Cyan(strings.Join(opts.Paths, ", ")))
	if len(opts.Namespaces) > 0 {
		nsStr := "namespace"
		if len(opts.Namespaces) > 1 {
			nsStr += "s"
		}
		fmt.Printf("  %s %s\n", cmdutil.Gray(nsStr+":"), cmdutil.Cyan(strings.Join(opts.Namespaces, ", ")))
	}
	if opts.Name != "" {
		fmt.Printf("  %s %s\n", cmdutil.Gray("name:"), cmdutil.Cyan(opts.Name))
	}
	if opts.Selector != "" {
		fmt.Printf("  %s %s\n", cmdutil.Gray("selector:"), cmdutil.Cyan(opts.Selector))
	}
	if opts.Kind != "" {
		fmt.Printf("  %s %s\n", cmdutil.Gray("kind:"), cmdutil.Cyan(opts.Kind))
	}
	fmt.Println()

	for _, r := range opts.Results {
		if r == nil {
			continue
		}

		var c migrateCounts
		for _, cr := range r.CRs {
			c.add(cr)
		}

		total := len(r.CRs)
		printKindHeader(r, total, opts)

		verboseShown := 0
		for _, cr := range r.CRs {
			ref := cr.Ref()
			switch {
			case cr.Err != nil:
				fmt.Printf("  %s %s  %s\n", cmdutil.FailureMark(), cmdutil.Red(ref), cmdutil.Red(cr.Err.Error()))
			case cr.Drifted:
				fmt.Printf("  %s %s  %s\n", cmdutil.Dim("~"), cmdutil.Gray(ref), cmdutil.Yellow("destination exists and differs — skipped"))
			case cr.Excluded:
				fmt.Printf("  %s %s\n", cmdutil.Dim("–"), cmdutil.Gray(ref+"  (not selected: migrate.selection when/or)"))
			case cr.Skipped:
				skippedMsg := "(already migrated)"
				if opts.Clean {
					skippedMsg = "(no annotations)"
				}
				fmt.Printf("  %s %s\n", cmdutil.Dim("–"), cmdutil.Gray(ref+"  "+skippedMsg))
			case len(cr.Fields) == 0 && cr.Pruned:
				fmt.Printf("  %s %s  %s\n", cmdutil.SuccessMark(), cmdutil.Cyan(ref), cmdutil.Gray("already migrated; "+pruneVerb(opts.DryRun)))
			default:
				var verb string
				switch {
				case opts.Clean && opts.DryRun:
					verb = "would clean"
				case opts.Clean:
					verb = "cleaned"
				case opts.DryRun:
					verb = "would migrate"
				default:
					verb = "migrated"
				}
				line := verb + ": " + strings.Join(cr.Fields, ", ")
				if r.To != "" && !opts.Clean {
					line = verb + " to " + r.To
				}
				if cr.Pruned {
					line += "; " + pruneVerb(opts.DryRun)
				}
				fmt.Printf("  %s %s  %s\n", cmdutil.SuccessMark(), cmdutil.Cyan(ref), cmdutil.Gray(line))
				if opts.Verbose && cr.AfterSpec != nil {
					if opts.Limit == 0 || verboseShown < opts.Limit {
						printSpec(cr.AfterSpec)
						verboseShown++
					}
				}
			}
		}

		if opts.Verbose && opts.Limit > 0 && verboseShown >= opts.Limit && c.migrated > opts.Limit {
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("… %d more (use --limit 0 to show all)", c.migrated-opts.Limit)))
		}

		fmt.Printf("\n  %s\n\n", buildMigrateSummary(c, total, opts.DryRun, opts.Clean))
	}
}

// migrateCounts buckets the CRs of one kind for the summary line.
type migrateCounts struct {
	migrated, skipped, excluded, drifted, pruned, failed int
}

func (c *migrateCounts) add(cr migrate.CRMigrateResult) {
	switch {
	case cr.Err != nil:
		c.failed++
		return
	case cr.Drifted:
		c.drifted++
	case cr.Excluded:
		c.excluded++
	case cr.Skipped:
		c.skipped++
	case len(cr.Fields) > 0:
		c.migrated++
	}
	if cr.Pruned {
		c.pruned++
	}
}

func pruneVerb(dryRun bool) string {
	if dryRun {
		return "would prune source"
	}
	return "source pruned"
}

// printKindHeader prints the kind line, and the scope the run used when it
// differs from the CLI flags (the blueprint's migrate.selection applied).
func printKindHeader(r *migrate.KindMigrateResult, total int, opts Opts) {
	countStr := fmt.Sprintf("%d CR", total)
	if total != 1 {
		countStr += "s"
	}
	name := cmdutil.Bold(r.Kind)
	if r.To != "" {
		name += cmdutil.Gray(" → ") + cmdutil.Bold(r.To)
	}
	fmt.Printf("%s  %s  %s\n", cmdutil.Cyan("◆"), name, cmdutil.Gray(countStr))

	flags := migrate.Selection{Namespaces: opts.Namespaces, Name: opts.Name, Selector: opts.Selector}
	if r.Selection.IsEmpty() || r.Selection.Equal(flags) {
		return
	}
	var parts []string
	if len(r.Selection.Namespaces) > 0 {
		parts = append(parts, "namespaces "+strings.Join(r.Selection.Namespaces, ","))
	}
	if r.Selection.Name != "" {
		parts = append(parts, "name "+r.Selection.Name)
	}
	if r.Selection.Selector != "" {
		parts = append(parts, "selector "+r.Selection.Selector)
	}
	fmt.Printf("   %s %s\n", cmdutil.Gray("scope:"), cmdutil.Cyan(strings.Join(parts, " · ")))
}

func buildMigrateSummary(c migrateCounts, total int, dryRun, clean bool) string {
	var verb string
	switch {
	case clean && dryRun:
		verb = "would be cleaned"
	case clean:
		verb = "cleaned"
	case dryRun:
		verb = "would be migrated"
	default:
		verb = "migrated"
	}
	parts := []string{fmt.Sprintf("%d/%d %s", c.migrated, total, verb)}
	if c.pruned > 0 {
		pruned := "sources pruned"
		if dryRun {
			pruned = "sources would be pruned"
		}
		parts = append(parts, cmdutil.Yellow(fmt.Sprintf("%d %s", c.pruned, pruned)))
	}
	if c.drifted > 0 {
		parts = append(parts, cmdutil.Yellow(fmt.Sprintf("%d drifted", c.drifted)))
	}
	if c.skipped > 0 {
		parts = append(parts, cmdutil.Gray(fmt.Sprintf("%d already done", c.skipped)))
	}
	if c.excluded > 0 {
		parts = append(parts, cmdutil.Gray(fmt.Sprintf("%d not selected", c.excluded)))
	}
	if c.failed > 0 {
		parts = append(parts, cmdutil.Red(fmt.Sprintf("%d failed", c.failed)))
	}
	mark := cmdutil.SuccessMark()
	if c.failed > 0 {
		mark = cmdutil.FailureMark()
	}
	return mark + " " + strings.Join(parts, " · ")
}

func printSpec(spec map[string]interface{}) {
	b, err := sigsyaml.Marshal(spec)
	if err != nil {
		return
	}
	fmt.Printf("    %s\n", cmdutil.Gray("spec:"))
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		fmt.Printf("      %s\n", cmdutil.Dim(line))
	}
}
