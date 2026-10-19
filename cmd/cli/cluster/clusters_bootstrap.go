//go:build !runtime && !gateway

package cluster

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"gopkg.in/yaml.v3"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/tools/cluster/bootstrap"
)

var clustersBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Provision least-privilege access on a target cluster for the gateway",
	Long: `Provision the ServiceAccount, ClusterRole, and token Secret on a target
cluster, then store the credentials in the gateway cluster so the gateway
can route applies to it.

The ClusterRole is scoped to exactly the serve-enabled CRDs declared in the
blueprint — no wildcard resources, no cluster-admin.

Examples:
  inrun clusters bootstrap --context kind-prod --name prod
  inrun clusters bootstrap --context kind-staging --name staging --namespace inrun
  inrun clusters bootstrap --context kind-prod --name prod --dry-run
  inrun clusters bootstrap --context kind-prod --name prod --emit-rbac
  inrun clusters bootstrap --config cluster-config.yaml
  inrun clusters bootstrap --validate cluster-config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		validatePath, _ := cmd.Flags().GetString("validate")
		configPath, _ := cmd.Flags().GetString("config")

		if validatePath != "" {
			return runBootstrapValidate(validatePath)
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		saNamespace, _ := cmd.Flags().GetString("sa-namespace")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		emitRBAC, _ := cmd.Flags().GetBool("emit-rbac")

		opts := bootstrap.RunOptions{
			Namespace:    namespace,
			DryRun:       dryRun,
			EmitRBACOnly: emitRBAC,
		}

		if configPath != "" {
			outPath, _ := cmd.Flags().GetString("out")
			return runBootstrapConfig(cmd, configPath, outPath, opts)
		}

		targetCtx, _ := cmd.Flags().GetString("context")
		name, _ := cmd.Flags().GetString("name")

		entry := bootstrap.ClusterEntry{
			Name:        name,
			Context:     targetCtx,
			SANamespace: saNamespace,
		}

		bp, err := cmdutil.BuildBlueprint(cmd)
		if err != nil {
			return err
		}

		return runBootstrapSingle(cmd.Context(), bp, entry, opts)
	},
}

// runBootstrapValidate loads and validates a config file without touching any cluster.
func runBootstrapValidate(path string) error {
	cfg, err := bootstrap.LoadConfig(path)
	if err != nil {
		return err
	}
	if err := bootstrap.ValidateConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("%s bootstrap config valid (%d cluster(s))\n", cmdutil.SuccessMark(), len(cfg.Clusters))
	for _, e := range cfg.Clusters {
		fmt.Printf("  %-12s →  %s\n", e.Name, e.Context)
	}
	return nil
}

// runBootstrapConfig bootstraps all clusters listed in the config file.
// No blueprint is required — rules come from each entry's rules: field (if any).
// When outPath is non-empty, the bootstrap results are written to a
// gateway.clusters-shaped YAML file suitable for `inrun clusters check --config`.
func runBootstrapConfig(cmd *cobra.Command, path, outPath string, opts bootstrap.RunOptions) error {
	cfg, err := bootstrap.LoadConfig(path)
	if err != nil {
		return err
	}
	if err := bootstrap.ValidateConfig(cfg); err != nil {
		return err
	}

	var results []*bootstrap.Result
	for _, entry := range cfg.Clusters {
		result, err := runBootstrapSingleResult(cmd.Context(), nil, entry, opts)
		if err != nil {
			return fmt.Errorf("cluster %q: %w", entry.Name, err)
		}
		if result != nil {
			results = append(results, result)
		}
	}

	if outPath != "" && len(results) > 0 {
		if err := bootstrap.WriteClusterCredentials(outPath, results); err != nil {
			return err
		}
		fmt.Printf("\n%s cluster credentials → %s\n", cmdutil.SuccessMark(), outPath)
		fmt.Printf("  %s inrun clusters check --config %s\n", cmdutil.Gray("→"), outPath)
	}
	return nil
}

// runBootstrapSingleResult is the result-returning variant used by runBootstrapConfig.
func runBootstrapSingleResult(ctx context.Context, bp *blueprint.Blueprint, entry bootstrap.ClusterEntry, opts bootstrap.RunOptions) (*bootstrap.Result, error) {
	fmt.Println()
	fmt.Printf("%s  inrun clusters bootstrap\n", cmdutil.Bold("⎈"))
	fmt.Printf("  %s cluster name:   %s\n", cmdutil.Gray("→"), cmdutil.Bold(entry.Name))
	fmt.Printf("  %s target context: %s\n", cmdutil.Gray("→"), cmdutil.Bold(entry.Context))
	fmt.Printf("  %s namespace:      %s\n", cmdutil.Gray("→"), cmdutil.Gray(opts.Namespace))
	fmt.Printf("  %s sa-namespace:   %s\n", cmdutil.Gray("→"), cmdutil.Gray(entry.SANamespace))
	if opts.DryRun {
		fmt.Printf("  %s\n", cmdutil.Yellow("dry-run: no changes will be made"))
	}
	fmt.Println()

	result, err := bootstrap.Cluster(ctx, bp, entry, opts, func(msg string) {
		fmt.Printf("   %s %s\n", cmdutil.SuccessMark(), msg)
	})
	if err != nil {
		return nil, fmt.Errorf("%s %s", cmdutil.FailureMark(), err.Error())
	}

	if opts.EmitRBACOnly {
		return nil, bootstrapPrintClusterRoleYAML(entry, result.ClusterRoleRules)
	}

	if opts.DryRun {
		fmt.Printf("  %s dry-run complete: no changes were made\n", cmdutil.Gray("○"))
		fmt.Println()
		return nil, nil
	}

	fmt.Println()
	bootstrapPrintBlueprintSnippet(entry.Name, result.Endpoint, result.SecretName, result.SecretNamespace, !result.HasCA)
	return result, nil
}

// runBootstrapSingle bootstraps one cluster entry and prints the appropriate output.
func runBootstrapSingle(ctx context.Context, bp *blueprint.Blueprint, entry bootstrap.ClusterEntry, opts bootstrap.RunOptions) error {
	fmt.Println()
	fmt.Printf("%s  inrun clusters bootstrap\n", cmdutil.Bold("⎈"))
	fmt.Printf("  %s cluster name:   %s\n", cmdutil.Gray("→"), cmdutil.Bold(entry.Name))
	fmt.Printf("  %s target context: %s\n", cmdutil.Gray("→"), cmdutil.Bold(entry.Context))
	fmt.Printf("  %s namespace:      %s\n", cmdutil.Gray("→"), cmdutil.Gray(opts.Namespace))
	fmt.Printf("  %s sa-namespace:   %s\n", cmdutil.Gray("→"), cmdutil.Gray(entry.SANamespace))
	if opts.DryRun {
		fmt.Printf("  %s\n", cmdutil.Yellow("dry-run: no changes will be made"))
	}
	fmt.Println()

	result, err := bootstrap.Cluster(ctx, bp, entry, opts, func(msg string) {
		fmt.Printf("   %s %s\n", cmdutil.SuccessMark(), msg)
	})
	if err != nil {
		return fmt.Errorf("%s %s", cmdutil.FailureMark(), err.Error())
	}

	if opts.EmitRBACOnly {
		return bootstrapPrintClusterRoleYAML(entry, result.ClusterRoleRules)
	}

	if opts.DryRun {
		fmt.Printf("  %s dry-run complete: no changes were made\n", cmdutil.Gray("○"))
		fmt.Println()
		return nil
	}

	fmt.Println()
	bootstrapPrintBlueprintSnippet(entry.Name, result.Endpoint, result.SecretName, result.SecretNamespace, !result.HasCA)
	return nil
}

// ── output helpers ────────────────────────────────────────────────────────────

func bootstrapPrintBlueprintSnippet(name, endpoint, secretName, namespace string, insecure bool) {
	fmt.Printf("%s  Add to your blueprint:\n\n", cmdutil.Bold("⎈"))
	if insecure {
		fmt.Printf(`gateway:
  clusters:
    %s:
      endpoint: %s
      tokenRef:
        name: %s
        namespace: %s
        key: token
      insecure: true
`, name, endpoint, secretName, namespace)
	} else {
		fmt.Printf(`gateway:
  clusters:
    %s:
      endpoint: %s
      tokenRef:
        name: %s
        namespace: %s
        key: token
      caRef:
        name: %s
        namespace: %s
        key: ca.crt
`, name, endpoint, secretName, namespace, secretName, namespace)
	}
	fmt.Println()
}

func bootstrapPrintClusterRoleYAML(entry bootstrap.ClusterEntry, rules []rbacv1.PolicyRule) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   bootstrap.ClusterRoleName(entry),
			Labels: bootstrap.Labels(),
		},
		Rules: rules,
	}
	b, err := yaml.Marshal(cr)
	if err != nil {
		return fmt.Errorf("marshaling ClusterRole: %w", err)
	}
	fmt.Printf("# ── ClusterRole (%s) ──────────────────────────────────────────────\n", bootstrap.ClusterRoleName(entry))
	fmt.Printf("---\n%s\n", string(b))
	return nil
}

func init() {
	clustersBootstrapCmd.Flags().String("context", "", "kubectl context for the TARGET cluster")
	clustersBootstrapCmd.Flags().String("name", "", "name for this cluster in gateway.clusters")
	clustersBootstrapCmd.Flags().String("namespace", "default", "namespace in the gateway cluster for the credential Secret")
	clustersBootstrapCmd.Flags().String("sa-namespace", bootstrap.DefaultSANamespace, "namespace on the TARGET cluster for the ServiceAccount and token Secret")
	clustersBootstrapCmd.Flags().String("config", "", "path to a cluster-config.yaml to bootstrap multiple clusters")
	clustersBootstrapCmd.Flags().StringP("out", "o", "", "write cluster credentials to this file after bootstrap (gateway.clusters format; use with check --config)")
	clustersBootstrapCmd.Flags().String("validate", "", "validate a cluster-config.yaml without connecting to any cluster")
	clustersBootstrapCmd.Flags().Bool("dry-run", false, "print what would be applied without making changes")
	clustersBootstrapCmd.Flags().Bool("emit-rbac", false, "print only the ClusterRole YAML for review")

	clustersCmd.AddCommand(clustersBootstrapCmd)
}
