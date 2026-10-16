//go:build !runtime

package gate

import (
	"github.com/intentd/inrun/cmd/cli/gate/output"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/spf13/cobra"
)

var gateMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate CR spec fields declared with migrate.from in the Catalog",
	Long: `Apply schema migrations to live CRs using migrate.from declarations in the Catalog.

For each serve field with migrate.from set, the migrator:
  1. reads the old value from the migrate.from spec path
  2. applies the field's value/values transform (if any)
  3. writes the result to the field's current spec path
  4. removes the migrate.from path

Migration is idempotent — CRs where migrate.from is absent are skipped.
Requires a live cluster connection.

Example:
  inrun gate migrate -f catalog.yaml
  inrun gate migrate -f catalog.yaml --dry-run
  inrun gate migrate -f catalog.yaml --kind WebApp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		clean, _ := cmd.Flags().GetBool("clean")
		strict, _ := cmd.Flags().GetBool("strict")
		verbose, _ := cmd.Flags().GetBool("verbose")
		limit, _ := cmd.Flags().GetInt("limit")
		pageSize, _ := cmd.Flags().GetInt64("page-size")
		kindFilter, _ := cmd.Flags().GetString("kind")
		name, _ := cmd.Flags().GetString("name")
		selector, _ := cmd.Flags().GetString("selector")
		namespaces, _ := cmd.Flags().GetStringSlice("namespace")

		if err := validateSelector(selector); err != nil {
			return err
		}

		paths, err := resolvePaths(cmd)
		if err != nil {
			return err
		}

		cat, kube, err := loadCatalogAndKube(paths)
		if err != nil {
			return err
		}

		migratorOpts := migrate.MigratorOpts{
			DryRun:     dryRun,
			Strict:     strict,
			PageSize:   pageSize,
			Name:       name,
			Selector:   selector,
			Namespaces: namespaces,
			Log:        logger.Nop(),
		}
		var migrator *migrate.Migrator
		if clean {
			migrator = migrate.New(cmd.Context(), kube, cat.UserNotes(), migrate.ModeClean, migratorOpts)
		} else {
			migrator = migrate.New(cmd.Context(), kube, cat.UserNotes(), migrate.ModeForward, migratorOpts)
		}

		opts := output.Opts{
			DryRun:     dryRun,
			Clean:      clean,
			Verbose:    verbose,
			Limit:      limit,
			Paths:      paths,
			Name:       name,
			Namespaces: namespaces,
			Selector:   selector,
			Kind:       kindFilter,
		}

		printFn, err := resolveMigrateOutputFormat(cmd, migrate.ModeForward.String())
		if err != nil {
			return err
		}

		return runMigratorForKinds(cat, migrator, opts, printFn)
	},
}

func init() {
	gatewayCmd.AddCommand(gateMigrateCmd)
	gateMigrateCmd.Flags().StringSliceP("file", "f", nil, "Path(s) to catalog.yaml (repeatable)")
	gateMigrateCmd.Flags().Bool("dry-run", false, "Preview changes without writing to the cluster")
	gateMigrateCmd.Flags().Bool("clean", false, "Remove migration annotations without touching the spec (discards the rollback safety net)")
	gateMigrateCmd.Flags().Bool("strict", false, "Fail if any individual CR cannot be migrated (default: warn and skip)")
	gateMigrateCmd.Flags().Bool("verbose", false, "Print the resulting spec for each migrated CR")
	gateMigrateCmd.Flags().IntP("limit", "l", 0, "Max CRs to show in verbose mode (0 = all)")
	gateMigrateCmd.Flags().Int64("page-size", 0, "CRs fetched per List call (0 = API server default, useful for testing pagination)")
	gateMigrateCmd.Flags().StringP("kind", "k", "", "Migrate only this kind (default: all kinds with migrate.from)")
	gateMigrateCmd.Flags().String("name", "", "Migrate only the CR with this name")
	gateMigrateCmd.Flags().String("selector", "", "Select CRs by label (Kubernetes label selector syntax)")
	gateMigrateCmd.Flags().StringSliceP("namespace", "n", nil, "Restrict migration to these namespaces (comma-separated; default: all)")
	gateMigrateCmd.Flags().StringP("output", "o", "text", "Output format: text, json")
}
