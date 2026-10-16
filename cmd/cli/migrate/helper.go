//go:build !runtime

package migrate

import (
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/migrate/output"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

// runMigrateCmd runs a migrate-family operation end to end: load the catalog,
// build a migrator for the given mode, build the output options, and render
// the result.
func runMigrateCmd(cmd *cobra.Command, mode migrate.MigrateMode) error {
	paths, err := resolvePaths(cmd)
	if err != nil {
		return err
	}

	cat, kube, err := loadCatalogAndKube(paths)
	if err != nil {
		return err
	}

	migrator, err := newMigrator(cmd, kube, cat.UserNotes(), mode)
	if err != nil {
		return err
	}

	opts := outputOptsFrom(cmd, paths)
	opts.Clean = mode == migrate.ModeClean

	printFn, err := resolveMigrateOutputFormat(cmd, mode.String())
	if err != nil {
		return err
	}

	return runMigratorForKinds(cat, migrator, opts, printFn)
}

// newMigrator builds a CLI migrator for the given mode. The CLI renders its
// own output, so the migrator logs nothing.
func newMigrator(cmd *cobra.Command, kube kubeclient.Interface, notes types.NoteRegistry, mode migrate.MigrateMode) (*migrate.Migrator, error) {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	strict, _ := cmd.Flags().GetBool("strict")
	pageSize, _ := cmd.Flags().GetInt64("page-size")
	name, _ := cmd.Flags().GetString("name")
	selector, _ := cmd.Flags().GetString("selector")
	namespaces, _ := cmd.Flags().GetStringSlice("namespace")
	if err := validateSelector(selector); err != nil {
		return nil, err
	}
	return migrate.New(cmd.Context(), kube, notes, mode, migrate.MigratorOpts{
		DryRun:     dryRun,
		Strict:     strict,
		PageSize:   pageSize,
		Name:       name,
		Selector:   selector,
		Namespaces: namespaces,
		Log:        logger.Nop(),
	}), nil
}

// outputOptsFrom builds the rendering options for a migrate-family command
// from its flags and the already-resolved catalog paths. It reads the same
// flags the migrator does — dry-run, verbose, limit, kind, name, selector,
// namespace — plus the paths resolved from --file.
func outputOptsFrom(cmd *cobra.Command, paths []string) output.Opts {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")
	limit, _ := cmd.Flags().GetInt("limit")
	name, _ := cmd.Flags().GetString("name")
	selector, _ := cmd.Flags().GetString("selector")
	kind, _ := cmd.Flags().GetString("kind")
	namespaces, _ := cmd.Flags().GetStringSlice("namespace")

	return output.Opts{
		DryRun:     dryRun,
		Verbose:    verbose,
		Limit:      limit,
		Paths:      paths,
		Namespaces: namespaces,
		Name:       name,
		Selector:   selector,
		Kind:       kind,
	}
}

// resolvePaths reads catalog file paths from the --file flag, falling back to
// defaults. Returns an error if no paths are found.
func resolvePaths(cmd *cobra.Command) ([]string, error) {
	paths, _ := cmd.Flags().GetStringSlice("file")
	if len(paths) == 0 {
		paths = cmdutil.DefaultFilePaths()
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf(cmdutil.ErrNoCatalog)
	}
	return paths, nil
}

// loadCatalogAndKube merges the given catalog paths, builds a scheme registry,
// and starts a live kube client. Returns the resolved catalog and a started client.
func loadCatalogAndKube(paths []string) (*catalog.Catalog, kubeclient.Interface, error) {
	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return nil, nil, fmt.Errorf("merging catalogs: %w", err)
	}
	cat, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing catalog: %w", err)
	}
	scheme, err := catalog.NewSchemeRegistry(cat)
	if err != nil {
		return nil, nil, fmt.Errorf("building scheme: %w", err)
	}
	kube := kubeclient.NewKubeclient(cmdutil.Cfg, scheme)
	if err := kube.Start(cmdutil.Ctx); err != nil {
		return nil, nil, fmt.Errorf("starting kubeclient: %w", err)
	}
	return cat, kube, nil
}

// runMigratorForKinds runs the migrator over every kind in cat, filtered by opts.Kind when set.
func runMigratorForKinds(cat *catalog.Catalog, migrator *migrate.Migrator, opts output.Opts, printFn func(output.Opts)) error {
	results, err := migrator.Run(cmdutil.Ctx, cat)
	for _, r := range results {
		if opts.Kind != "" && r.Kind != opts.Kind {
			continue
		}
		opts.Results = append(opts.Results, r)
	}
	if err != nil {
		printFn(opts)
		return err
	}
	printFn(opts)
	return nil
}

// validateSelector returns the parse error when sel is not a valid label
// selector, and nil otherwise.
func validateSelector(sel string) error {
	_, err := labels.ValidateLabelSelector(sel)
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
