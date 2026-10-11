//go:build !runtime && !gateway

package self

import (
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Inrun version",
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		short, _ := cmd.Flags().GetBool("short")

		if verbose {
			fmt.Println("Inrun")
			fmt.Printf("%-12s %s\n", "Version:", version.Short())
			fmt.Printf("%-12s %s\n", "Commit:", version.Commit)
			fmt.Printf("%-12s %s\n", "Built:", version.Date)
			return
		}

		if short {
			fmt.Println("inrun", version.Short())
			return
		}

		fmt.Printf(
			"inrun %s (commit %s, built %s)\n",
			version.Short(),
			version.Commit,
			version.Date,
		)
	},
}

func init() {
	cmdutil.RootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolP("short", "s", false, "Show short version for Inrun")

	// Shadow global flags so they don't appear under `inrun version`
	cmdutil.ShadowGlobalCommandFlags(versionCmd, "file")
}
