//go:build gateway

package gate

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/cmd/internal"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/spf13/cobra"
)

// runGateway is the gateway build's root command: bare `inrun` runs the
// gateway (TLS and admission webhooks, cluster only).
func runGateway(cmd *cobra.Command, args []string) error {
	paths, _ := cmd.Flags().GetStringSlice("file")
	if len(paths) == 0 {
		paths = cmdutil.DefaultFilePaths()
	}
	if len(paths) == 0 {
		paths = cmdutil.Kfg.Catalog().Paths()
	}
	if len(paths) == 0 {
		return fmt.Errorf(cmdutil.ErrNoCatalog)
	}

	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging catalogs: %w", err)
	}

	logger.Debug().
		Str("catalogs", strings.Join(paths, ", ")).
		Int("total", m.Count()).
		Int("enabled", m.EnabledCount()).
		Msg("catalogs merged")

	internal.RunGateway(cmdutil.Kfg, m, cmdutil.Ctx)
	return nil
}

func init() {
	cmdutil.RootCmd.Args = cobra.NoArgs
	cmdutil.RootCmd.RunE = runGateway
}
