//go:build !runtime && !gateway

package generate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/tools/generate"
	"github.com/spf13/cobra"
)

// bundleOptsFromFor reads the --for flag and returns the corresponding BundleOptions.
// --for accepts a comma-separated list of component names: runtime (alias: run),
// gateway (alias: gw), console.
// When --for is absent or empty, all three components are included (default).
func bundleOptsFromFor(cmd *cobra.Command) (generate.BundleOptions, error) {
	forVal, _ := cmd.Flags().GetString("for")
	if forVal == "" {
		return generate.DefaultBundleOptions(), nil
	}
	opts := generate.BundleOptions{}
	var unknown []string
	for _, part := range strings.Split(forVal, ",") {
		name := strings.TrimSpace(strings.ToLower(part))
		if name == "" {
			continue
		}
		switch name {
		case "run", "runtime":
			opts.IncludeRuntime = true
		case "gw", "gateway":
			opts.IncludeGateway = true
		case "console":
			opts.IncludeConsole = true
		default:
			unknown = append(unknown, part)
		}
	}
	if len(unknown) > 0 {
		return generate.BundleOptions{}, fmt.Errorf(
			"inrun: unknown --for value(s): %s\n\nValid values are:\n"+
				"  runtime   (alias: run)          — reconcilers, leader election\n"+
				"  gateway   (alias: gw)            — TLS, admission webhooks\n"+
				"  console   — console\n\n"+
				"Example: --for gateway\n"+
				"         --for runtime,console",
			strings.Join(unknown, ", "),
		)
	}
	if !opts.IncludeRuntime && !opts.IncludeGateway && !opts.IncludeConsole {
		return generate.BundleOptions{}, fmt.Errorf(
			"inrun: --for produced an empty component list; nothing to generate\n\n" +
				"Valid values are: runtime (run), gateway (gw), console",
		)
	}
	return opts, nil
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Inrun components",
}

// buildCatalogFromPath builds an expanded Catalog from a file path without
// requiring a cobra command context. Useful when the path is already known.
func buildCatalogFromPath(path string) (*catalog.Catalog, error) {
	m := merger.New(path)
	if err := m.Merge(); err != nil {
		return nil, fmt.Errorf("merging catalog: %w", err)
	}
	return pipeline.BuildExpanded(cmdutil.Kfg, m)
}

var generateRbacCmd = &cobra.Command{
	Use:   "rbac",
	Short: "Generate RBAC ClusterRoles and ServiceAccounts for Inrun components",
	Long: `Reads one or more catalog.yaml files, merges them, and generates minimal
ClusterRoles for the runtime and gateway processes, plus ServiceAccounts for
all three components (runtime, gateway, console).

Use --for to limit the output to specific components. By default all three
are included. Multiple values are comma-separated.

Examples:
  inrun generate rbac -f catalog.yaml
  inrun generate rbac -f catalog.yaml --for gateway
  inrun generate rbac -f catalog.yaml --for runtime,console
  inrun generate rbac -f a.yaml,b.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := cmdutil.GenerateCatalog(cmd)
		if err != nil {
			return err
		}
		namespace, _ := cmd.Flags().GetString("namespace")
		outputFile, _ := cmd.Flags().GetString("output")

		k, err := pipeline.BuildExpanded(cmdutil.Kfg, out.Merger)
		if err != nil {
			return fmt.Errorf("build catalog: %w", err)
		}

		log.Println("generating rbac...")

		opts, err := bundleOptsFromFor(cmd)
		if err != nil {
			return err
		}

		if !k.IsGatewayEnabled() {
			opts.IncludeGateway = false
		}
		runtimeRules := k.GenerateRuntimeRBACRules()
		gatewayRules := k.GenerateGatewayRBACRules()

		output, err := generate.RBACWithOptions(runtimeRules, gatewayRules, opts, namespace, outputFile)
		if err != nil {
			return fmt.Errorf("generate rbac: %w", err)
		}

		if err := cmdutil.WriteOutput(outputFile, "rbac.yaml", []byte(output)); err != nil {
			return err
		}

		return writeClusterRBACFiles(k, outputFile)
	},
}

var generateConfigMapCmd = &cobra.Command{
	Use:   "configmap",
	Short: "Generate a ConfigMap embedding a Catalog or Stack",
	Long: `Reads a catalog.yaml or stack.yaml file and produces a ConfigMap
that embeds the file under data:<filename>. Useful for injecting Catalogs
into the in-cluster Inrun runtime.

Example:
  inrun generate configmap -f catalog.yaml
  inrun generate configmap -f stack.yaml -n inrun-system -o out.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := cmdutil.GenerateCatalog(cmd)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		outputFile, _ := cmd.Flags().GetString("output")

		k, err := pipeline.BuildExpanded(cmdutil.Kfg, out.Merger)
		if err != nil {
			return fmt.Errorf("build catalog: %w", err)
		}

		log.Println("generating configmap...")

		expanded, err := k.SerializeExpanded()
		if err != nil {
			return fmt.Errorf("serialize catalog: %w", err)
		}

		cm, err := generate.ConfigMap(expanded, namespace)
		if err != nil {
			return fmt.Errorf("generate configmap: %w", err)
		}

		return cmdutil.WriteOutput(outputFile, "config.yaml", cm)
	},
}

var generateBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Generate a complete installation bundle (RBAC + ConfigMap)",
	Long: `Generates a complete Inrun installation bundle containing:
  • Namespace (default: 'inrun-system')
  • ServiceAccounts for runtime, gateway, and console
  • ClusterRoles and ClusterRoleBindings (one per process, minimal permissions)
  • ConfigMap embedding your Catalog

Use --for to limit the output to specific components. By default all three
are included. Multiple values are comma-separated.

Examples:
  inrun generate bundle -f catalog.yaml
  inrun generate bundle -f catalog.yaml --for gateway
  inrun generate bundle -f catalog.yaml --for runtime,console
  inrun generate bundle -f catalog.yaml -o bundle.yaml -n inrun-system`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := cmdutil.GenerateCatalog(cmd)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		workloadNamespace, _ := cmd.Flags().GetString("workload-namespace")
		outputFile, _ := cmd.Flags().GetString("output")

		k, err := pipeline.BuildExpanded(cmdutil.Kfg, out.Merger)
		if err != nil {
			return fmt.Errorf("build catalog: %w", err)
		}

		opts, err := bundleOptsFromFor(cmd)
		if err != nil {
			return err
		}

		log.Println("generating bundle...")

		if !k.IsGatewayEnabled() {
			opts.IncludeGateway = false
		}
		runtimeRules := k.GenerateRuntimeRBACRules()
		gatewayRules := k.GenerateGatewayRBACRules()

		expanded, err := k.SerializeExpanded()
		if err != nil {
			return fmt.Errorf("serialize catalog: %w", err)
		}

		bundle, err := generate.RenderBundle(runtimeRules, gatewayRules, expanded, namespace, workloadNamespace, opts)
		if err != nil {
			return fmt.Errorf("generate bundle: %w", err)
		}

		if err := cmdutil.WriteOutput(outputFile, "bundle.yaml", []byte(bundle)); err != nil {
			return err
		}

		return writeClusterRBACFiles(k, outputFile)
	},
}

// writeClusterRBACFiles generates a gateway-<name>-rbac.yaml for each remote cluster
// that has serve-enabled CRDs routed to it. Files land in the same directory as
// outputFile (or the current directory when outputFile is empty or "-").
// Template-routed CRDs appear in every cluster file; a warning is printed for those.
func writeClusterRBACFiles(k *catalog.Catalog, outputFile string) error {
	clusterRules, templateKinds := k.GenerateGatewayClusterRBACRules()
	if len(clusterRules) == 0 {
		return nil
	}
	if outputFile == "-" {
		return nil // stdout mode — no path to place cluster files alongside
	}

	dir := clusterOutputDir(outputFile)
	clusters := k.GatewayClusters()

	if len(templateKinds) > 0 {
		crdTxt := "CRDs"
		if len(templateKinds) == 1 {
			crdTxt = "CRD"
		}

		fmt.Fprintf(os.Stderr,
			"\n%s warning: template-routed %s added to all cluster RBAC files: %s\n"+
				"  Remove rules for clusters that should not have access.\n",
			cmdutil.WarningMark(), crdTxt, strings.Join(templateKinds, ", "),
		)
	}

	fmt.Println()
	for _, name := range cmdutil.SortedKeys(clusterRules) {
		b, err := generate.RBACForCluster(name, clusterRules[name], "kube-system")
		if err != nil {
			return fmt.Errorf("generate cluster rbac [%s]: %w", name, err)
		}
		filename := "gateway-" + name + "-rbac.yaml"
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, b, 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		log.Printf("%s generated successfully\n", filename)
		if cfg, ok := clusters[name]; ok {
			fmt.Printf("  Apply to cluster %q (%s):\n    kubectl apply -f %s\n\n", name, cfg.EndpointURL(), path)
		}
	}
	return nil
}

// clusterOutputDir returns the directory in which per-cluster RBAC files should
// be written, mirroring the logic in writeOutput.
func clusterOutputDir(outputFile string) string {
	if outputFile == "" || outputFile == "-" {
		return "."
	}
	info, err := os.Stat(outputFile)
	if err == nil && info.IsDir() {
		return outputFile
	}
	return filepath.Dir(outputFile)
}

func init() {
	cmdutil.RootCmd.AddCommand(generateCmd)

	generateCmd.AddCommand(generateRegistryCmd)
	generateCmd.AddCommand(generateRbacCmd)
	generateCmd.AddCommand(generateConfigMapCmd)
	generateCmd.AddCommand(generateBundleCmd)

	// All three commands use StringSliceP so generateCatalog can read them uniformly.
	for _, cmd := range []*cobra.Command{generateConfigMapCmd, generateBundleCmd, generateRbacCmd} {
		cmd.Flags().StringSliceP("file", "f", []string{}, "Path to catalog.yaml or stack.yaml (repeatable or comma-separated)")
	}

	generateRegistryCmd.Flags().StringP("dirs", "d", "", "Comma-separated list of project directories to generate registries for")
	generateRegistryCmd.Flags().Duration("fetch-timeout", 2*time.Minute, "Timeout to fetch Go hook or constructor from 'location'")

	// Shared flags for all file-consuming generate commands.
	for _, cmd := range []*cobra.Command{
		generateRegistryCmd,
		generateRbacCmd,
		generateConfigMapCmd,
		generateBundleCmd,
	} {
		cmd.Flags().Bool("dry-run", false, "Print generated output to stdout without writing files")
		cmd.Flags().StringP("output", "o", "", "Write generated output to file")
		cmd.Flags().StringP("namespace", "n", cmdutil.DefaultNamespace(), "Namespace for the ServiceAccount")
	}

	// bundle-only flags
	generateBundleCmd.Flags().StringP("workload-namespace", "w", "", "Extra namespace to create in the bundle for the operator's workloads")

	// component-selection flag (shared by rbac and bundle)
	// --for runtime          → runtime SA + ClusterRole only
	// --for gateway          → gateway SA + ClusterRole only
	// --for runtime,gateway  → both, no console SA
	// --for runtime,console       → runtime + console SA, no gateway
	// (absent)               → all three (default)
	for _, cmd := range []*cobra.Command{generateRbacCmd, generateBundleCmd} {
		cmd.Flags().String("for", "", "Limit output to specific components: runtime, gateway, console (comma-separated; default: all)")
	}

	// Shadow global flags so they don't appear under `inrun generate`
	cmdutil.ShadowGlobalCommandFlags(generateCmd)
	cobra.MarkFlagRequired(generateCmd.Flags(), "catalog")
}
