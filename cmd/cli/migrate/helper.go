//go:build !runtime

package migrate

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/migrate/output"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

// runMigrateCmd runs a migrate-family operation end to end: load the blueprint,
// build a migrator for the given mode, build the output options, and render
// the result.
func runMigrateCmd(cmd *cobra.Command, mode migrate.MigrateMode) error {
	paths, err := resolvePaths(cmd)
	if err != nil {
		return err
	}

	adjust, err := migrateOverrides(cmd, paths)
	if err != nil {
		return err
	}
	bp, kube, err := pipeline.BootstrapWith(cmd.Context(), cmdutil.Cfg, paths, adjust)
	if err != nil {
		return err
	}

	migrator, migrateOpts, err := newMigrator(cmd, kube, bp.UserFunctions(), mode)
	if err != nil {
		return err
	}
	prog := newProgress(cmd, mode)
	defer prog.stop()
	migrator.Progress = prog.callback()
	migrateOpts.Progress = migrator.Progress

	if mode == migrate.ModeForward && !migrateOpts.DryRun {
		if yes, _ := cmd.Flags().GetBool("yes"); !yes {
			ok, err := confirmPrune(cmd.Context(), bp, kube, bp.UserFunctions(), migrateOpts, prog)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		}
	}

	opts := outputOptsFrom(cmd, paths, migrateOpts)
	opts.Clean = mode == migrate.ModeClean

	printFn, err := resolveMigrateOutputFormat(cmd, mode.String())
	if err != nil {
		return err
	}

	prog.begin()
	return runMigratorForKinds(cmd.Context(), bp, migrator, opts, printFn, prog)
}

// newMigrator builds a CLI migrator for the given mode. The CLI renders its
// own output, so the migrator logs nothing.
func newMigrator(cmd *cobra.Command, kube kubeclient.Interface, funcs types.KELFunctionRegistry, mode migrate.MigrateMode) (*migrate.Migrator, migrate.MigratorOpts, error) {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	strict, _ := cmd.Flags().GetBool("strict")
	pageSize, _ := cmd.Flags().GetInt64("page-size")
	kind, _ := cmd.Flags().GetString("kind")
	name, _ := cmd.Flags().GetString("name")
	selector, _ := cmd.Flags().GetString("selector")
	namespaces, _ := cmd.Flags().GetStringSlice("namespace")
	force, _ := cmd.Flags().GetBool("force")
	if _, err := labels.ValidateLabelSelector(selector); err != nil {
		return nil, migrate.MigratorOpts{}, err
	}

	opts := migrate.MigratorOpts{
		DryRun:     dryRun,
		Strict:     strict,
		PageSize:   pageSize,
		Kind:       kind,
		Name:       name,
		Selector:   selector,
		Namespaces: namespaces,
		Force:      force,
		Log:        logger.Nop(),
	}
	return migrate.New(cmd.Context(), kube, funcs, mode, opts), opts, nil
}

// outputOptsFrom builds the rendering options for a migrate-family command
// from its flags and the already-resolved blueprint paths. It reads the same
// flags the migrator does — dry-run, verbose, limit, kind, name, selector,
// namespace — plus the paths resolved from --file.
func outputOptsFrom(cmd *cobra.Command, paths []string, migrator migrate.MigratorOpts) output.Opts {
	kind, _ := cmd.Flags().GetString("kind")
	verbose, _ := cmd.Flags().GetBool("verbose")
	limit, _ := cmd.Flags().GetInt("limit")

	return output.Opts{
		Kind:       kind,
		DryRun:     migrator.DryRun,
		Namespaces: migrator.Namespaces,
		Name:       migrator.Name,
		Selector:   migrator.Selector,
		Verbose:    verbose,
		Limit:      limit,
		Paths:      paths,
	}
}

// resolvePaths reads blueprint file paths from the --file flag, falling back to
// defaults. Returns an error if no paths are found.
func resolvePaths(cmd *cobra.Command) ([]string, error) {
	paths, _ := cmd.Flags().GetStringSlice("file")
	if len(paths) == 0 {
		paths = cmdutil.DefaultFilePaths()
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf(cmdutil.ErrNoBlueprint)
	}
	return paths, nil
}

// runMigratorForKinds runs the migrator over bp and prints the results, also
// when the run fails part way.
func runMigratorForKinds(ctx context.Context, bp *blueprint.Blueprint, migrator *migrate.Migrator, opts output.Opts, printFn func(output.Opts), prog *progress) error {
	results, err := migrator.Run(ctx, bp)
	prog.stop()
	opts.Results = results
	printFn(opts)
	return err
}

// resolveMigrateOutputFormat maps the --output flag to the printFn for a
// migrate or rollback run: "text" (default) for the human printer, "json"
// for the machine-readable one. mode selects which pair of printers to use.
func resolveMigrateOutputFormat(cmd *cobra.Command, mode string) (func(output.Opts), error) {
	outputFormat, _ := cmd.Flags().GetString("output")
	switch outputFormat {
	case "text", "":
		return textPrinterFor(mode), nil
	case "json":
		return jsonPrinterFor(mode), nil
	default:
		return nil, fmt.Errorf("unknown output format %q (want text or json)", outputFormat)
	}
}

// textPrinterFor returns the human printer for the given mode.
// mode is "migrate" or "rollback" — the same string the CLI's subcommand uses.
func textPrinterFor(mode string) func(output.Opts) {
	switch mode {
	case "rollback":
		return output.PrintRollback
	default:
		return output.PrintMigrate
	}
}

// jsonPrinterFor returns the JSON printer for the given mode.
func jsonPrinterFor(mode string) func(output.Opts) {
	switch mode {
	case "rollback":
		return output.PrintRollbackJSON
	default:
		return output.PrintMigrateJSON
	}
}

// migrateOverrides turns --set into a step on the merged Blueprint: each key is
// set under the migrate block of the kinds --kind or --all names, before the
// Blueprint is built and validated. It needs one of the two, so a --set never
// changes a kind nobody named. Nil without --set; rollback and clean have none.
func migrateOverrides(cmd *cobra.Command, paths []string) (func(*merger.Merger) error, error) {
	raw, err := cmd.Flags().GetStringArray("set")
	if err != nil || len(raw) == 0 {
		return nil, nil
	}
	sets, err := merger.ParseSets(raw)
	if err != nil {
		return nil, err
	}
	kind, _ := cmd.Flags().GetString("kind")
	all, _ := cmd.Flags().GetBool("all")
	switch {
	case kind == "" && !all:
		return nil, fmt.Errorf("--set needs --kind <Kind> or --all (-A): which kinds it changes")
	case kind != "" && all:
		return nil, fmt.Errorf("--kind and --all are mutually exclusive")
	}

	// The kinds are known once the Blueprint is validated, so find the entries
	// on a first load, then change them on the load that runs.
	bp, _, err := pipeline.LoadBlueprint(cmdutil.Cfg, paths)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		// Only entries with a kind-level migrate block: --set keys are relative to
		// it, and a destination of the same kind (a rescope) has none.
		if entry.HasMigrate() && (all || entry.Kind() == kind) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		if kind != "" {
			return nil, fmt.Errorf("--set: no CRD of kind %q declares a kind migration (migrate.to)", kind)
		}
		return nil, fmt.Errorf("--set --all: no CRD declares a kind migration (migrate.to)")
	}

	prefixed := make([]merger.Set, len(sets))
	for i, s := range sets {
		prefixed[i] = merger.Set{Path: "migrate." + s.Path, Value: s.Value}
	}
	return func(m *merger.Merger) error {
		for _, name := range names {
			if err := m.Override(name, prefixed); err != nil {
				return err
			}
		}
		return nil
	}, nil
}
