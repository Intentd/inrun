//go:build !runtime && !gateway

package testsuite

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/validate"

	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/util/fileio"

	"github.com/spf13/cobra"
)

var e2eCmd = &cobra.Command{
	Use:   "e2e",
	Short: "Run declarative end-to-end tests against a real cluster",
	Long: `Runs an E2E test defined in a YAML spec file.

Orchestrates the full lifecycle: cluster creation → dependency installation →
CRD apply → bundle apply → Inrun install → CR apply → expectation checking → cleanup.

The same command runs locally and in CI. The e2e.yaml file is the source of truth.

  inrun e2e                 # e2e.yaml here, then test/e2e.yaml
  inrun e2e -f e2e.yaml
  inrun e2e -f e2e.yaml --keep-cluster
  inrun e2e -f e2e.yaml --cluster my-existing-context
  inrun e2e -f e2e.yaml --version v1.2.3 --values values.yaml

Discovery mode — runs all *e2e.yaml files found recursively (skips pure aggregators):

  inrun e2e ./...
  inrun e2e ./examples/reconcilers/declarative/...
  inrun e2e ./... --wait 2s
  inrun e2e ./... --skip vendor,testdata,external/07-vault`,
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		// Allow positional argument: inrun e2e ./... (like go test ./...)
		if len(args) > 0 {
			file = args[0]
		}
		if file == "" {
			file = cmdutil.FindFile(cmdutil.FileE2e)
		}
		if file == "" {
			return fmt.Errorf("no %s found here or in %s/ — pass -f <file>", cmdutil.FileE2e, cmdutil.DirTest)
		}

		e2eOpts := e2e.Options{}
		var err error
		e2eOpts.KeepCluster, _ = cmd.Flags().GetBool("keep-cluster")
		e2eOpts.UseCurrentCtx, _ = cmd.Flags().GetBool("use-current")
		e2eOpts.ClusterCtx, _ = cmd.Flags().GetString("cluster")
		e2eOpts.Workers, _ = cmd.Flags().GetInt("workers")
		e2eOpts.InrunVersion, _ = cmd.Flags().GetString("version")
		e2eOpts.InrunChart, err = chartFlag(cmd)
		if err != nil {
			return err
		}
		e2eOpts.ValueFiles, _ = cmd.Flags().GetStringSlice("values")
		helmArgRaw, _ := cmd.Flags().GetStringSlice("set")
		e2eOpts.HelmArgs = cmdutil.HelmSetArgs(helmArgRaw)
		e2eOpts.DevServer, _ = cmd.Flags().GetBool("dev-server")
		e2eOpts.NoRuntime, _ = cmd.Flags().GetBool("no-runtime")
		e2eOpts.Command = "inrun " + strings.Join(os.Args[1:], " ")
		e2eOpts.MaxFailures, _ = cmd.Flags().GetInt("max-failures")
		e2eOpts.ReportFile, _ = cmd.Flags().GetString("report")
		if e2eOpts.ReportFile == "-" {
			e2eOpts.ReportFile = cmdutil.FileReport
		}
		// --dry-run alone is client: print what would run, like --validate.
		// --dry-run=server runs the suite's writes as server dry runs.
		dryRunMode, _ := cmd.Flags().GetString("dry-run")
		switch dryRunMode {
		case "", "client":
		case "server":
			if !e2eOpts.UseCurrentCtx && e2eOpts.ClusterCtx == "" {
				if !cluster.ClusterReachable() {
					return fmt.Errorf("--dry-run=server sends writes to an existing cluster: none is reachable; add --use-current or --cluster <context>")
				}
				// A reachable current context is the cluster a server dry run checks against.
				e2eOpts.UseCurrentCtx = true
			}
			e2eOpts.DryRun = true
		default:
			return fmt.Errorf("--dry-run must be client or server, got %q", dryRunMode)
		}
		dryRun := dryRunMode == "client"

		// CLI-specific
		wait, _ := cmd.Flags().GetString("wait")
		skip, _ := cmd.Flags().GetStringSlice("skip")
		limit, _ := cmd.Flags().GetInt("limit")

		validateOnly, _ := cmd.Flags().GetBool("validate")
		listOnly, _ := cmd.Flags().GetBool("list")

		action, err := pickAction(file, validateOnly, listOnly, dryRun)
		if err != nil {
			return err
		}

		switch action {
		case actionList:
			root, _ := fileio.IsDiscoveryArg(file)
			return listE2EDiscovery(root, skip, limit)

		case actionValidate:
			if root, ok := fileio.IsDiscoveryArg(file); ok {
				return validateE2EDiscovery(root, skip)
			}
			return validate.ValidateE2EFile(file)

		case actionDryRun:
			// Print what would run: the expectations for one file, the
			// discovered files for ./...
			if root, ok := fileio.IsDiscoveryArg(file); ok {
				return listE2EDiscovery(root, skip, limit)
			}
			return validate.ValidateE2EFile(file)
		}

		// Discovery mode: inrun e2e ./... or -f ./some/path/...
		if root, ok := fileio.IsDiscoveryArg(file); ok {
			return runE2EDiscovery(cmd, root, wait, skip, e2eOpts)
		}

		if simulate.IsSimulateDoc(file) {
			return errors.Join(
				fmt.Errorf("%s is a Simulate file, not an E2E file", file),
				fmt.Errorf("run it in memory:        inrun simulate -f %s", file),
				fmt.Errorf("run it against envtest:  inrun simulate -f %s --envtest", file),
				fmt.Errorf("or generate an E2E:      inrun e2e init"),
			)
		}

		if err := validate.ValidateE2EFileQuiet(file); err != nil {
			return err
		}

		if e2eOpts.UseCurrentCtx {
			if !cluster.ClusterReachable() {
				return fmt.Errorf("cannot reach a Kubernetes cluster. " +
					"\n  Check your kubeconfig, or run without '--use-current'.")
			}
		}

		if ctx := e2eOpts.ClusterCtx; ctx != "" {
			if !cluster.CheckContext(ctx) {
				return fmt.Errorf("Kubernetes context %q does not exist in the current kubeconfig", ctx)
			}
		}

		runner, err := e2e.New(file, e2eOpts)
		if err != nil {
			return err
		}
		_, err = runner.Run(cmd.Context())
		return err
	},
}

func init() {
	cmdutil.RootCmd.AddCommand(e2eCmd)
	e2eCmd.AddCommand(e2eInitCmd)

	e2eInitCmd.Flags().StringP("file", "f", "", "Path to blueprint.yaml or stack.yaml")
	e2eInitCmd.Flags().Bool("force", false, "Overwrite existing e2e.yaml")
	e2eInitCmd.Flags().Bool("dry-run", false, "Print the generated e2e.yaml to stdout instead of writing the file")
	e2eCmd.Flags().Bool("validate", false, "Validate the spec without running it. Single file: validate it. ./...: validate every discovered file.")
	e2eCmd.Flags().BoolP("list", "l", false, "List the files ./... would run, without running them.")
	e2eCmd.Flags().String("dry-run", "", "client (the default): print what would run without running it. server: send the suite's kubectl writes to an existing cluster as server dry runs, and skip its commands and assertions.")
	e2eCmd.Flags().Lookup("dry-run").NoOptDefVal = "client"
	e2eInitCmd.Flags().Bool("suite", false, "Aggregate all e2e.yaml leaf files found under the given dir (default: .)")
	e2eInitCmd.Flags().String("wait", "5s", "Pause injected between suite imports (first excluded). Only applies with --suite.")
	e2eInitCmd.Flags().StringSlice("skip", []string{}, "Comma-separated path patterns to exclude from suite discovery")

	e2eCmd.Flags().StringP("file", "f", "", "Path to the E2E spec file, or ./... for discovery (default: e2e.yaml here, then test/e2e.yaml)")
	e2eCmd.Flags().Bool("keep-cluster", false, "Keep the kind cluster after the test completes")
	e2eCmd.Flags().Bool("use-current", false, "Use the current kubectl context, skip cluster creation")
	e2eCmd.Flags().String("cluster", "", "Use an existing kubectl context instead of creating a cluster")
	e2eCmd.Flags().Int("workers", 0, "Number of kind worker nodes to provision (default: 0, control-plane only)")
	e2eCmd.Flags().String("version", "", "Inrun version to install (e.g., v1.2.3)")
	e2eCmd.Flags().String("chart", "", "Local Inrun chart directory to install instead of the published chart (e.g. ./charts/inrun)")
	e2eCmd.Flags().StringSlice("values", []string{}, "Helm values files to pass to Inrun installation")
	e2eCmd.Flags().StringSlice("set", []string{}, "Additional Helm --set arguments (e.g., key=value)")
	e2eCmd.Flags().Bool("dev-server", false, "Deploy the mock dev server into the cluster for external: examples")
	e2eCmd.Flags().Bool("no-runtime", false, "Skip the Inrun runtime; start the gateway only. CR is optional.")
	e2eCmd.Flags().String("wait", "", "Duration to wait between discovered tests (e.g. 2s). Only applies in ./... discovery mode.")
	e2eCmd.Flags().StringSlice("skip", []string{}, "Comma-separated path patterns to skip during ./... discovery (e.g. vendor,testdata)")
	e2eCmd.Flags().StringP("report", "r", "", "Write test results as Markdown or JSON based on the file extension. Results are always printed to stdout.")
	e2eCmd.Flags().Lookup("report").NoOptDefVal = "-"
	e2eCmd.Flags().Int("limit", 10, "maximum number of discovered e2e files to list in --list (0 = no limit)")
	e2eCmd.Flags().Int("max-failures", 0, "Stop the run after N failed expectations (0 = run all)")

	// Shadow global flags so they don't appear under `inrun e2e`
	cmdutil.ShadowGlobalCommandFlags(e2eCmd)
}

// chartFlag returns --chart as an absolute chart directory, or "" when unset.
func chartFlag(cmd *cobra.Command) (string, error) {
	chart, _ := cmd.Flags().GetString("chart")
	return cmdutil.ChartDir(chart)
}
