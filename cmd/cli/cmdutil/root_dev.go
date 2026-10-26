//go:build !runtime && !gateway

package cmdutil

import "github.com/spf13/cobra"

func init() {
	// SilenceUsage is an option to silence usage when an error occurs.
	RootCmd.SilenceUsage = true
	// SilenceErrors is an option to quiet errors down stream.
	RootCmd.SilenceErrors = true

	RootCmd.CompletionOptions.DisableDefaultCmd = false

	// Dev-only persistent flags — not needed in the production runtime binary.
	RootCmd.PersistentFlags().String("kubeconfig", "", "Path to kubeconfig file")
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "Show full context")

}

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Inrun infrastructure resources",
}
