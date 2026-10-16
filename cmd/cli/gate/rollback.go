//go:build !runtime

package gate

import (
	"github.com/intentd/inrun/cmd/cli/gate/output"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/spf13/cobra"
)

var gateRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back CRs to their pre-migration state using the stored snapshot",
	Long: `Reverse a schema migration by restoring the field values captured in the
inrun.dev/migrate-snapshot annotation.

For each CR with a snapshot annotation, the rollback:
  1. reads the recorded {before, after} entry for each field from the snapshot
  2. compares the current spec value at the new path (path:) to the recorded
     after value — if they differ the field was modified after migration and is
     left untouched (reported as drifted)
  3. writes the before value back to the old path (migrate.from)
  4. nulls out the new path (path:) and removes the migration annotations

CRs without a snapshot annotation are skipped. Rollback is idempotent.
Requires a live cluster connection.

Example:
  inrun gate rollback -f catalog.yaml
  inrun gate rollback -f catalog.yaml --dry-run
  inrun gate rollback -f catalog.yaml --namespace platform-01 --name my-app`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
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

		migrator := migrate.New(cmd.Context(), kube, cat.UserNotes(), migrate.ModeRollback, migrate.MigratorOpts{
			DryRun:     dryRun,
			Strict:     strict,
			PageSize:   pageSize,
			Name:       name,
			Selector:   selector,
			Namespaces: namespaces,
			Log:        logger.Nop(),
		})

		opts := output.Opts{
			DryRun:     dryRun,
			Verbose:    verbose,
			Limit:      limit,
			Paths:      paths,
			Namespaces: namespaces,
			Name:       name,
			Selector:   selector,
			Kind:       kindFilter,
		}

		printFn, err := resolveMigrateOutputFormat(cmd, migrate.ModeRollback.String())
		if err != nil {
			return err
		}
		return runMigratorForKinds(cat, migrator, opts, printFn)
	},
}

func init() {
	gatewayCmd.AddCommand(gateRollbackCmd)
	gateRollbackCmd.Flags().StringSliceP("file", "f", nil, "Path(s) to catalog.yaml (repeatable)")
	gateRollbackCmd.Flags().Bool("dry-run", false, "Preview changes without writing to the cluster")
	gateRollbackCmd.Flags().Bool("strict", false, "Fail if any individual CR cannot be rolled back (default: warn and skip)")
	gateRollbackCmd.Flags().Bool("verbose", false, "Print the resulting spec for each rolled-back CR")
	gateRollbackCmd.Flags().IntP("limit", "l", 0, "Max CRs to show in verbose mode (0 = all)")
	gateRollbackCmd.Flags().Int64("page-size", 0, "CRs fetched per List call (0 = API server default)")
	gateRollbackCmd.Flags().StringP("kind", "k", "", "Roll back only this kind (default: all kinds with migrate.from)")
	gateRollbackCmd.Flags().String("name", "", "Roll back only the CR with this name")
	gateRollbackCmd.Flags().String("selector", "", "Select CRs by label (Kubernetes label selector syntax)")
	gateRollbackCmd.Flags().StringSliceP("namespace", "n", nil, "Restrict rollback to these namespaces (comma-separated; default: all)")
	gateRollbackCmd.Flags().StringP("output", "o", "text", "Output format: text, json")
}
