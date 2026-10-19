//go:build !runtime && !gateway

package output

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
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

		migrated, skipped, failed := 0, 0, 0
		for _, cr := range r.CRs {
			switch {
			case cr.Err != nil:
				failed++
			case cr.Skipped:
				skipped++
			default:
				migrated++
			}
		}

		total := len(r.CRs)
		countStr := fmt.Sprintf("%d CR", total)
		if total != 1 {
			countStr += "s"
		}
		fmt.Printf("%s  %s  %s\n", cmdutil.Cyan("◆"), cmdutil.Bold(r.Kind), cmdutil.Gray(countStr))

		verboseShown := 0
		for _, cr := range r.CRs {
			ref := cr.Ref()
			switch {
			case cr.Err != nil:
				fmt.Printf("  %s %s  %s\n", cmdutil.FailureMark(), cmdutil.Red(ref), cmdutil.Red(cr.Err.Error()))
			case cr.Skipped:
				skippedMsg := "(already migrated)"
				if opts.Clean {
					skippedMsg = "(no annotations)"
				}
				fmt.Printf("  %s %s\n", cmdutil.Dim("–"), cmdutil.Gray(ref+"  "+skippedMsg))
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
				fmt.Printf("  %s %s  %s\n", cmdutil.SuccessMark(), cmdutil.Cyan(ref), cmdutil.Gray(verb+": "+strings.Join(cr.Fields, ", ")))
				if opts.Verbose && cr.AfterSpec != nil {
					if opts.Limit == 0 || verboseShown < opts.Limit {
						printSpec(cr.AfterSpec)
						verboseShown++
					}
				}
			}
		}

		if opts.Verbose && opts.Limit > 0 && verboseShown >= opts.Limit && migrated > opts.Limit {
			fmt.Printf("    %s\n", cmdutil.Gray(fmt.Sprintf("… %d more (use --limit 0 to show all)", migrated-opts.Limit)))
		}

		fmt.Printf("\n  %s\n\n", buildMigrateSummary(migrated, skipped, failed, total, opts.DryRun, opts.Clean))
	}
}

func buildMigrateSummary(migrated, skipped, failed, total int, dryRun, clean bool) string {
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
	parts := []string{fmt.Sprintf("%d/%d %s", migrated, total, verb)}
	if skipped > 0 {
		parts = append(parts, cmdutil.Gray(fmt.Sprintf("%d already done", skipped)))
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
