//go:build !runtime

package migrate

import (
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove migration annotations without touching the spec",
	Long: `Remove the inrun.dev/migrate-snapshot and inrun.dev/migrate-timestamp
annotations from CRs that carry them. The spec is left untouched.

Run this after a migration is confirmed stable to discard the rollback
safety net and release the annotation space. It is a one-way operation:
once the snapshot is removed, rollback is no longer possible for that CR.

CRs without migration annotations are skipped. Clean is idempotent.
Requires a live cluster connection.

Example:
  inrun migrate clean -f blueprint.yaml
  inrun migrate clean -f blueprint.yaml --dry-run
  inrun migrate clean -f blueprint.yaml --namespace platform-01 --name my-app`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateCmd(cmd, migrate.ModeClean)
	},
}

func init() {
	migrateCmd.AddCommand(cleanCmd)
}
