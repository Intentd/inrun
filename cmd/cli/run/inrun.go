//go:build runtime

package run

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/cmd/internal"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/spf13/cobra"
)

// runRuntime is the runtime build's root command: bare `inrun` runs the
// runtime. Without -f it reads catalog.yaml (or stack.yaml) from the current
// directory.
func runRuntime(cmd *cobra.Command, args []string) error {
	// Resolve catalog paths
	paths, _ := cmd.Flags().GetStringSlice("file")
	paths, err := cmdutil.ResolveCatalogPaths(paths)
	if err != nil {
		return err
	}

	// Merge catalogs
	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging catalogs: %w", err)
	}

	logger.Debug().
		Str("catalogs", strings.Join(paths, ", ")).
		Int("total", m.Count()).
		Int("enabled", m.EnabledCount()).
		Msg("catalogs merged")

	internal.RunRuntime(cmdutil.Kfg, m, cmdutil.Ctx)
	return nil
}

func init() {
	cmdutil.RootCmd.Args = cobra.NoArgs
	cmdutil.RootCmd.RunE = runRuntime
}
