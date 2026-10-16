//go:build !runtime

package migrate

import (
	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/migrate"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
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

Subcommands:
  rollback   Restore CRs from their migration snapshots
  clean      Remove migration annotations without touching the spec

Example:
  inrun migrate -f catalog.yaml
  inrun migrate -f catalog.yaml --dry-run
  inrun migrate -f catalog.yaml --kind WebApp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrateCmd(cmd, migrate.ModeForward)
	},
}

func init() {
	cmdutil.RootCmd.AddCommand(migrateCmd)

	// Persistent flags are inherited by every subcommand (rollback, clean).
	// Declare shared options here so a new subcommand picks them up without
	// re-registering, and so `inrun migrate --help` lists them once.
	migrateCmd.PersistentFlags().StringSliceP("file", "f", nil, "Path(s) to catalog.yaml (repeatable)")
	migrateCmd.PersistentFlags().Bool("dry-run", false, "Preview changes without writing to the cluster")
	migrateCmd.PersistentFlags().Bool("strict", false, "Fail if any individual CR cannot be migrated (default: warn and skip)")
	migrateCmd.PersistentFlags().Bool("verbose", false, "Print the resulting spec for each migrated CR")
	migrateCmd.PersistentFlags().Int("limit", 0, "Max CRs to show in verbose mode (0 = all)")
	migrateCmd.PersistentFlags().Int64("page-size", 0, "CRs fetched per List call (0 = API server default, useful for testing pagination)")
	migrateCmd.PersistentFlags().StringP("kind", "k", "", "Migrate only this kind (default: all kinds with migrate.from)")
	migrateCmd.PersistentFlags().String("name", "", "Migrate only the CR with this name")
	migrateCmd.PersistentFlags().String("selector", "", "Select CRs by label (Kubernetes label selector syntax)")
	migrateCmd.PersistentFlags().StringSliceP("namespace", "n", nil, "Restrict migration to these namespaces (comma-separated; default: all)")
	migrateCmd.PersistentFlags().StringP("output", "o", "text", "Output format: text, json")

	// Shadow global flags so they don't appear under `inrun version`
	cmdutil.ShadowGlobalCommandFlags(migrateCmd)
}
