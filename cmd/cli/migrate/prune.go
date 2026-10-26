//go:build !runtime

package migrate

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/term"
)

// confirmPrune asks before a forward run deletes source CRs. It dry-runs the
// migration to count the sources each kind would prune, shows the counts and
// asks [y/N] on stderr. It returns false when the user declines. Without a
// terminal it fails and asks for --yes instead of waiting for input.
func confirmPrune(ctx context.Context, bp *blueprint.Blueprint, kube kubeclient.Interface, funcs types.KELFunctionRegistry, opts migrate.MigratorOpts, prog *progress) (bool, error) {
	if !declaresPrune(bp, opts.Kind) {
		return true, nil
	}

	preview := opts
	preview.DryRun = true
	prog.start("Checking what prune would delete…")
	results, err := migrate.New(ctx, kube, funcs, migrate.ModeForward, preview).Run(ctx, bp)
	prog.stop()
	if err != nil {
		return false, err
	}

	var lines []string
	total := 0
	for _, r := range results {
		n := 0
		for _, cr := range r.CRs {
			if cr.Err == nil && cr.Pruned {
				n++
			}
		}
		if n == 0 {
			continue
		}
		total += n
		lines = append(lines, fmt.Sprintf("    %s → %s  %s", cmdutil.Bold(r.Kind), cmdutil.Bold(r.To), cmdutil.Yellow(plural(n, "source"))))
	}
	if total == 0 {
		return true, nil
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, cmdutil.Yellow("⚠ migrate.prune is on.")+" These source CRs are deleted after their destination is written:")
	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
	fmt.Fprintln(os.Stderr)

	if !term.IsTerminal() {
		return false, fmt.Errorf("prune needs confirmation: rerun with --yes, or --dry-run to preview")
	}
	return term.Confirm(fmt.Sprintf("Delete %s after migrating?", plural(total, "source CR")))
}

// declaresPrune reports whether any CRD in scope declares migrate.prune.
func declaresPrune(bp *blueprint.Blueprint, kind string) bool {
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok || !entry.HasMigrate() || !entry.Migrate.Prune {
			continue
		}
		if kind == "" || entry.GVK().Kind == kind {
			return true
		}
	}
	return false
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
