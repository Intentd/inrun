//go:build !runtime && !gateway

package cluster

import (
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Inrun infrastructure resources",
}

var deleteClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Delete one or more local kind clusters created by inrun create cluster",
	Long: `Deletes one or more local kind cluster by name.
	Separated by commas.


  inrun delete cluster
  inrun delete cluster --name inrun-e2e
  inrun delete cluster --name inrun-1,inrun-2,inrun-3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		nameList := cmdutil.SplitCommaSeparated(name)
		for _, n := range nameList {
			fmt.Printf("→ Deleting cluster '%s'...\n", n)
			if err := cluster.DeleteKindCluster(n); err != nil {
				return err
			}
			fmt.Printf("Cluster '%s' deleted.\n", n)
		}
		return nil
	},
}

func init() {
	cmdutil.RootCmd.AddCommand(deleteCmd)
	deleteCmd.AddCommand(deleteClusterCmd)

	deleteClusterCmd.Flags().StringP("name", "n", "inrun-playground", "Cluster name. Accepts multiple names separated by commas.")

	// Shadow global flags
	cmdutil.ShadowGlobalCommandFlags(deleteClusterCmd, "file")
}
