//go:build !runtime

package migrate

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/types"
	xterm "golang.org/x/term"
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

	if !stdinIsTerminal() {
		return false, fmt.Errorf("prune needs confirmation: rerun with --yes, or --dry-run to preview")
	}
	return askYes(os.Stdin, fmt.Sprintf("Delete %s after migrating? [y/N]: ", plural(total, "source CR")))
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

// askYes prints prompt to stderr and reports whether the answer is y or yes.
func askYes(in io.Reader, prompt string) (bool, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read answer: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// stdinIsTerminal reports whether stdin is a TTY. A char-device check is
// not enough: /dev/null is a char device too.
func stdinIsTerminal() bool {
	return xterm.IsTerminal(int(os.Stdin.Fd()))
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
