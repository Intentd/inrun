//go:build !runtime

package migrate

import (
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back CRs to their pre-migration state using the stored snapshot",
	Long: `Reverse a schema migration by restoring the field values captured in the
inrun.dev/migrate-field-snapshot annotation. For a migrate.to migration, the
destination's inrun.dev/migrate-kind-snapshot is used: the source is recreated
if it was pruned, then the destination is deleted.

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
  inrun migrate rollback -f blueprint.yaml
  inrun migrate rollback -f blueprint.yaml --dry-run
  inrun migrate rollback -f blueprint.yaml --namespace platform-01 --name my-app`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateCmd(cmd, migrate.ModeRollback)
	},
}

func init() {
	migrateCmd.AddCommand(rollbackCmd) // parent, not RootCmd
}
