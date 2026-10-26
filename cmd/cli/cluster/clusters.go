//go:build !runtime && !gateway

package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate"
	apigateway "github.com/intentd/inrun/pkg/gateway/api"
	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

// ── inrun clusters ──────────────────────────────────────────────────────────────

var clustersCmd = &cobra.Command{
	Use:   "clusters",
	Short: "List registered gateway clusters",
	Long: `List all clusters registered in gateway.clusters.

Shows the name, endpoint, and credential form for each cluster.
Reads the blueprint from the current directory or --file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bp, err := cmdutil.BuildBlueprint(cmd)
		if err != nil {
			return err
		}
		return runClustersList(bp)
	},
}

func runClustersList(bp *blueprint.Blueprint) error {
	fmt.Println()

	if !bp.IsGatewayEnabled() {
		fmt.Printf("  %s gateway is not enabled\n", cmdutil.Gray("○"))
		fmt.Println()
		return nil
	}

	clusters := bp.GatewayClusters()
	if len(clusters) == 0 {
		fmt.Printf("  %s gateway.clusters is empty — no remote clusters registered\n", cmdutil.Gray("○"))
		fmt.Println()
		return nil
	}

	fmt.Printf("  gateway.clusters (%d registered)\n", len(clusters))
	fmt.Println()

	for _, name := range cmdutil.SortedKeys(clusters) {
		cfg := clusters[name]
		fmt.Printf("  %s  %s\n", cmdutil.Cyan("→"), cmdutil.Bold(name))
		fmt.Printf("     %s\n", cmdutil.Gray(cfg.EndpointURL()))
		fmt.Printf("     %s\n", cmdutil.Gray(credentialSummary(cfg)))
		fmt.Println()
	}
	return nil
}

// ── inrun clusters validate ─────────────────────────────────────────────────────

var clustersValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate gateway.clusters configuration offline",
	Long: `Validate the gateway.clusters block in the blueprint.

Reads the blueprint and checks each cluster entry for structural validity:
endpoint required, exactly one credential form, required secret refs present.

Also verifies that every static serve.cluster and target.cluster reference
resolves to a registered cluster name. Template expressions are validated
against the full user-defined funcMap.

With --full, also reports which CRDs route to each cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")

		bp, err := cmdutil.BuildBlueprint(cmd)
		if err != nil {
			return err
		}

		return runClustersValidate(bp, full)
	},
}

func runClustersValidate(bp *blueprint.Blueprint, full bool) error {
	fmt.Println()
	fmt.Printf("%s  inrun clusters validate\n", cmdutil.Bold("⎈"))
	fmt.Println()

	if !bp.IsGatewayEnabled() {
		fmt.Printf("  %s gateway is not enabled — no clusters to validate\n", cmdutil.Gray("○"))
		fmt.Println()
		return nil
	}

	clusters := bp.GatewayClusters()
	if len(clusters) == 0 {
		fmt.Printf("  %s gateway.clusters is empty — no remote clusters registered\n", cmdutil.Gray("○"))
		fmt.Println()
		return nil
	}

	clusterRefs := buildClusterRefIndex(bp)

	fmt.Printf("  gateway.clusters (%d registered)\n", len(clusters))
	fmt.Println()

	allOK := true
	for _, name := range cmdutil.SortedKeys(clusters) {
		cfg := clusters[name]
		if !printClusterValidation(name, cfg, clusterRefs[name], full) {
			allOK = false
		}
	}

	if err := validate.ValidateGatewayClusters(bp); err != nil {
		fmt.Printf("  %s %s\n", cmdutil.FailureMark(), cmdutil.Red(err.Error()))
		allOK = false
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	if allOK {
		fmt.Printf("%s %d cluster(s) valid\n", cmdutil.SuccessMark(), len(clusters))
	} else {
		fmt.Printf("%s cluster validation failed\n", cmdutil.FailureMark())
		return fmt.Errorf("cluster validation failed")
	}
	fmt.Println()
	return nil
}

// ── inrun clusters check ────────────────────────────────────────────────────────

var clustersCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Connect to each registered cluster and verify CRD presence",
	Long: `Go online: read each cluster's credential secret from the management
cluster, connect to the remote cluster, verify the blueprint's CRDs are installed,
and report pass / unreachable / missing-CRD per cluster.

Uses the current kubectl context to reach the management cluster.
Pass --context <ctx> to use a specific context instead.

With --config <file>, skips the blueprint entirely and checks connectivity only
for each cluster listed in the bootstrap config file. Use this to verify
contexts are reachable before bootstrapping them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterCtx, _ := cmd.Flags().GetString("context")
		clustersRaw, _ := cmd.Flags().GetString("clusters")
		configPath, _ := cmd.Flags().GetString("config")

		if configPath != "" {
			clusters, err := cluster.LoadClustersFile(configPath)
			if err != nil {
				return err
			}
			return runClustersCheck(cmd.Context(), nil, clusters, clusterCtx)
		}

		bp, err := cmdutil.BuildBlueprint(cmd)
		if err != nil {
			return err
		}

		all := bp.GatewayClusters()
		if len(all) == 0 {
			fmt.Printf("  %s gateway.clusters is empty — nothing to check\n", cmdutil.Gray("○"))
			return nil
		}

		// Filter to requested subset when --clusters is set.
		clusters := all
		if clustersRaw != "" {
			selected := map[string]types.GatewayClusterConfig{}
			for _, name := range strings.Split(clustersRaw, ",") {
				name = strings.TrimSpace(name)
				cfg, ok := all[name]
				if !ok {
					return fmt.Errorf("%s cluster %q is not registered in gateway.clusters", cmdutil.FailureMark(), name)
				}
				selected[name] = cfg
			}
			clusters = selected
		}

		return runClustersCheck(cmd.Context(), bp, clusters, clusterCtx)
	},
}

func runClustersCheck(ctx context.Context, bp *blueprint.Blueprint, clusters map[string]types.GatewayClusterConfig, clusterCtx string) error {
	fmt.Println()
	fmt.Printf("%s  inrun clusters check\n", cmdutil.Bold("⎈"))
	fmt.Println()

	localKube, err := cluster.LocalClient(ctx, clusterCtx)
	if err != nil {
		return fmt.Errorf("%s management cluster: %w", cmdutil.FailureMark(), err)
	}

	results := apigateway.CheckClusters(ctx, bp, clusters, localKube, "default")

	anyFailed := false
	for _, r := range results {
		fmt.Printf("  %s  %s  %s\n", cmdutil.Cyan("→"), cmdutil.Bold(r.Name), cmdutil.Gray(r.Endpoint))

		if r.CredErr != nil {
			fmt.Printf("     %s credentials: %s\n", cmdutil.FailureMark(), cmdutil.Red(r.CredErr.Error()))
			anyFailed = true
			fmt.Println()
			continue
		}
		fmt.Printf("     %s credentials: read ok\n", cmdutil.SuccessMark())

		if r.ConnErr != nil {
			fmt.Printf("     %s connect: %s\n", cmdutil.FailureMark(), cmdutil.Red(r.ConnErr.Error()))
			anyFailed = true
			fmt.Println()
			continue
		}
		fmt.Printf("     %s connect: reachable\n", cmdutil.SuccessMark())

		for _, crdName := range cmdutil.SortedKeys(r.CRDs) {
			if err := r.CRDs[crdName]; err != nil {
				fmt.Printf("     %s crd %s: %s\n", cmdutil.FailureMark(), cmdutil.Red(crdName), cmdutil.Gray(err.Error()))
				anyFailed = true
			} else {
				fmt.Printf("     %s crd %s: installed\n", cmdutil.SuccessMark(), crdName)
			}
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("─", 60))
	if anyFailed {
		fmt.Printf("%s one or more clusters failed\n", cmdutil.FailureMark())
		return fmt.Errorf("one or more clusters failed checks")
	}
	fmt.Printf("%s all clusters reachable\n", cmdutil.SuccessMark())
	fmt.Println()
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func printClusterValidation(name string, cfg types.GatewayClusterConfig, refs []string, full bool) bool {
	fmt.Printf("  %s  %s\n", cmdutil.Cyan("→"), cmdutil.Bold(name))
	ok := true

	if cfg.EndpointURL() == "" {
		fmt.Printf("     %s endpoint: missing\n", cmdutil.FailureMark())
		ok = false
	} else {
		fmt.Printf("     %s endpoint: %s\n", cmdutil.SuccessMark(), cmdutil.Gray(cfg.EndpointURL()))
	}

	switch cfg.CredentialForm() {
	case "kubeconfig":
		fmt.Printf("     %s credential: kubeconfig %s\n", cmdutil.SuccessMark(),
			cmdutil.Gray(fmt.Sprintf("(secretRef: %s[%s])", cfg.SecretRef.SecretName(), cfg.SecretRef.SecretKey())))
	case "token":
		if cfg.IsInsecure() {
			fmt.Printf("     %s credential: bearer token %s\n", cmdutil.SuccessMark(),
				cmdutil.Gray(fmt.Sprintf("(tokenRef: %s[%s], insecure)", cfg.TokenRef.SecretName(), cfg.TokenRef.SecretKey())))
		} else {
			fmt.Printf("     %s credential: bearer token + CA %s\n", cmdutil.SuccessMark(),
				cmdutil.Gray(fmt.Sprintf("(tokenRef: %s[%s], caRef: %s[%s])",
					cfg.TokenRef.SecretName(), cfg.TokenRef.SecretKey(),
					cfg.CARef.SecretName(), cfg.CARef.SecretKey())))
		}
	default:
		fmt.Printf("     %s credential: no credential form declared\n", cmdutil.FailureMark())
		ok = false
	}

	if full {
		if len(refs) == 0 {
			fmt.Printf("     %s routes: %s\n", cmdutil.Gray("○"), cmdutil.Gray("not referenced by any CRD"))
		} else {
			fmt.Printf("     %s routes: %s\n", cmdutil.Gray("○"), cmdutil.Gray(strings.Join(refs, ", ")))
		}
	}

	fmt.Println()
	return ok
}

func credentialSummary(cfg types.GatewayClusterConfig) string {
	switch cfg.CredentialForm() {
	case "kubeconfig":
		return fmt.Sprintf("kubeconfig  secretRef: %s[%s]", cfg.SecretRef.SecretName(), cfg.SecretRef.SecretKey())
	case "token":
		if cfg.IsInsecure() {
			return fmt.Sprintf("token (insecure)  tokenRef: %s[%s]", cfg.TokenRef.SecretName(), cfg.TokenRef.SecretKey())
		}
		return fmt.Sprintf("token + CA  tokenRef: %s[%s]", cfg.TokenRef.SecretName(), cfg.TokenRef.SecretKey())
	default:
		return "no credential declared"
	}
}

func buildClusterRefIndex(bp *blueprint.Blueprint) map[string][]string {
	idx := map[string][]string{}
	for crdName, entry := range bp.EnabledCRDs() {
		if entry.Serve == nil {
			continue
		}
		for _, v := range entry.Serve.Clusters {
			if !types.IsTemplate(v) {
				idx[v] = append(idx[v], crdName+".serve.clusters")
			}
		}
		for targetName, target := range entry.Serve.Target.Entries {
			for _, v := range target.TargetClusters() {
				if !types.IsTemplate(v) {
					idx[v] = append(idx[v], fmt.Sprintf("%s.serve.target.%s.clusters", crdName, targetName))
				}
			}
		}
	}
	return idx
}

func init() {
	clustersValidateCmd.Flags().Bool("full", false, "Show CRD routing references per cluster")
	clustersCheckCmd.Flags().String("context", "", "kubectl context for reading credential secrets (defaults to current context)")
	clustersCheckCmd.Flags().String("clusters", "", "comma-separated list of cluster names to check (default: all registered clusters)")
	clustersCheckCmd.Flags().String("config", "", "path to a cluster-config.yaml to check connectivity without a blueprint")

	clustersCmd.AddCommand(clustersValidateCmd)
	clustersCmd.AddCommand(clustersCheckCmd)
	cmdutil.RootCmd.AddCommand(clustersCmd)
	cmdutil.ShadowGlobalCommandFlags(clustersCmd)
}
