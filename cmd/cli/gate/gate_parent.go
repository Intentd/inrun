//go:build !runtime

package gate

import (
	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/spf13/cobra"
)

// gatewayCmd is the "gate" parent command. It is available in both dev and
// gateway builds (//go:build !runtime) so that subcommands like "migrate"
// work in both environments.
var gatewayCmd = &cobra.Command{
	Use:   "gate",
	Short: "Gateway commands — admission evaluation, local dev server, schema migration",
}

func init() {
	cmdutil.RootCmd.AddCommand(gatewayCmd)
	gatewayCmd.Flags().StringSliceP("file", "f", nil, "Path(s) to blueprint.yaml (repeatable)")
}
