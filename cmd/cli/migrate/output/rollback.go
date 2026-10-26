//go:build !runtime && !gateway

package output

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
)

func PrintRollback(opts Opts) {
	fmt.Println()
	label := cmdutil.Bold("⟳") + "  inrun migrate rollback"
	if opts.DryRun {
		label += "  " + cmdutil.Yellow("[dry-run]")
	}
	fmt.Println(label)
	fmt.Printf("  %s %s\n", cmdutil.Gray("blueprint:"), cmdutil.Cyan(strings.Join(opts.Paths, ", ")))
	if len(opts.Namespaces) > 0 {
		nsStr := "namespace"
		if len(opts.Namespaces) != 1 {
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

		rolledBack, skipped, drifted, allDrifted, failed := 0, 0, 0, 0, 0
		for _, cr := range r.CRs {
			switch {
			case cr.Err != nil:
				failed++
			case cr.Skipped:
				skipped++
			case cr.AllDrifted:
				allDrifted++
			default:
				if len(cr.Fields) > 0 {
					rolledBack++
				}
				if len(cr.DriftedFields) > 0 {
					drifted++
				}
			}
		}

		total := len(r.CRs)
		printKindHeader(r, total, opts)

		verboseShown := 0
		for _, cr := range r.CRs {
			ref := cr.Ref()
			switch {
			case cr.Err != nil:
				fmt.Printf("  %s %s  %s\n", cmdutil.FailureMark(), cmdutil.Red(ref), cmdutil.Red(cr.Err.Error()))
			case cr.Skipped:
				fmt.Printf("  %s %s\n", cmdutil.Dim("–"), cmdutil.Gray(ref+"  (no snapshot)"))
			case cr.AllDrifted && r.To != "":
				fmt.Printf("  %s %s  %s\n", cmdutil.Dim("~"), cmdutil.Gray(ref), cmdutil.Yellow("destination modified after migration — left in place"))
			default:
				if len(cr.Fields) > 0 {
					verb := "rolled back"
					if opts.DryRun {
						verb = "would roll back"
					}
					line := verb + ": " + strings.Join(cr.Fields, ", ")
					if r.To != "" {
						line = verb + " to " + r.Kind + "; " + r.To + " deleted"
						if opts.DryRun {
							line = verb + " to " + r.Kind + "; would delete " + r.To
						}
					}
					fmt.Printf("  %s %s  %s\n", cmdutil.SuccessMark(), cmdutil.Cyan(ref), cmdutil.Gray(line))
					if opts.Verbose && cr.AfterSpec != nil {
						if opts.Limit == 0 || verboseShown < opts.Limit {
							printSpec(cr.AfterSpec)
							verboseShown++
						}
					}
				}
				for _, f := range cr.DriftedFields {
					fmt.Printf("  %s %s  %s\n", cmdutil.Dim("~"), cmdutil.Gray(ref), cmdutil.Dim(f+": drifted — modified after migration, skipped"))
				}
			}
		}

		if opts.Verbose && opts.Limit > 0 && verboseShown >= opts.Limit && rolledBack > opts.Limit {
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("… %d more (use --limit 0 to show all)", rolledBack-opts.Limit)))
		}

		fmt.Printf("\n  %s\n\n", buildRollbackSummary(rolledBack, skipped, drifted, allDrifted, failed, total, opts.DryRun))
	}
}

func buildRollbackSummary(rolledBack, skipped, drifted, allDrifted, failed, total int, dryRun bool) string {
	verb := "rolled back"
	if dryRun {
		verb = "would be rolled back"
	}
	parts := []string{fmt.Sprintf("%d/%d %s", rolledBack, total, verb)}
	if drifted > 0 {
		parts = append(parts, cmdutil.Yellow(fmt.Sprintf("%d drifted", drifted)))
	}
	if allDrifted > 0 {
		parts = append(parts, cmdutil.Yellow(fmt.Sprintf("%d all fields drifted", allDrifted)))
	}
	if skipped > 0 {
		parts = append(parts, cmdutil.Gray(fmt.Sprintf("%d no snapshot", skipped)))
	}
	if failed > 0 {
		parts = append(parts, cmdutil.Red(fmt.Sprintf("%d failed", failed)))
	}
	mark := cmdutil.SuccessMark()
	if failed > 0 {
		mark = cmdutil.FailureMark()
	}
	return mark + " " + strings.Join(parts, " · ")
}
