//go:build !runtime && !gateway

package cluster

import (
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/spf13/cobra"
)

var createClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Create a local kind cluster for Inrun development or testing",
	Long: `Creates a local kind cluster and switches kubectl to its context.
Downloads kind automatically if not found in PATH.

  inrun create cluster
  inrun create cluster --name inrun-e2e
  inrun create cluster --name inrun --count 3     # creates inrun-1, inrun-2, inrun-3
  inrun create cluster --provider kind --name my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		provider, _ := cmd.Flags().GetString("provider")
		workers, _ := cmd.Flags().GetInt("workers")
		version, _ := cmd.Flags().GetString("version")
		count, _ := cmd.Flags().GetInt("count")

		if provider != "kind" {
			return fmt.Errorf("provider %q not supported — only 'kind' is available", provider)
		}

		names := []string{name}
		if count > 1 {
			names = make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("%s-%d", name, i+1)
			}
		}

		for _, n := range names {
			fmt.Printf("→ Creating cluster '%s'...\n", n)
			if err := cluster.EnsureKindCluster(n, workers, version); err != nil {
				return err
			}
			fmt.Printf("\nCluster '%s' is ready.\n", n)
		}
		if count > 1 {
			fmt.Printf("%d clusters created.\n", count)
		}
		fmt.Printf("kubectl is now pointing to kind-%s.\n", names[len(names)-1])
		return nil
	},
}

func init() {
	cmdutil.RootCmd.AddCommand(cmdutil.CreateCmd)
	cmdutil.CreateCmd.AddCommand(createClusterCmd)

	createClusterCmd.Flags().StringP("name", "n", "inrun-playground", "Cluster name")
	createClusterCmd.Flags().StringP("provider", "p", "kind", "Cluster provider (only 'kind' is supported)")
	createClusterCmd.Flags().IntP("workers", "w", 0, "Number of kind worker nodes (default: 0, control-plane only)")
	createClusterCmd.Flags().StringP("version", "v", "", "kind version to use (default: "+cluster.DefaultKindVersion+")")
	createClusterCmd.Flags().IntP("count", "c", 1, "Number of clusters to create; names get a -1/-2/-3 suffix")

	// Shadow global flags
	cmdutil.ShadowGlobalCommandFlags(cmdutil.CreateCmd, "file")
}
