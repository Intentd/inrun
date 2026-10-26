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
// runtime. Without -f it reads blueprint.yaml (or stack.yaml) from the current
// directory.
func runRuntime(cmd *cobra.Command, args []string) error {
	// Resolve blueprint paths
	paths, _ := cmd.Flags().GetStringSlice("file")
	paths, err := cmdutil.ResolveBlueprintPaths(paths)
	if err != nil {
		return err
	}

	// Merge blueprints
	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging blueprints: %w", err)
	}

	logger.Debug().
		Str("blueprints", strings.Join(paths, ", ")).
		Int("total", m.Count()).
		Int("enabled", m.EnabledCount()).
		Msg("blueprints merged")

	internal.RunRuntime(cmdutil.Cfg, m, cmdutil.Ctx)
	return nil
}

func init() {
	cmdutil.RootCmd.Args = cobra.NoArgs
	cmdutil.RootCmd.RunE = runRuntime
}
