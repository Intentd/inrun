//go:build !runtime && !gateway

package testsuite

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/validate"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry/e2e"
	"gopkg.in/yaml.v3"

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
		keepCluster, _ := cmd.Flags().GetBool("keep-cluster")
		useCurrentCtx, _ := cmd.Flags().GetBool("use-current")
		clusterCtx, _ := cmd.Flags().GetString("cluster")
		workers, _ := cmd.Flags().GetInt("workers")
		version, _ := cmd.Flags().GetString("version")
		chart, err := chartFlag(cmd)
		if err != nil {
			return err
		}
		valuesFiles, _ := cmd.Flags().GetStringSlice("values")
		helmArgRaw, _ := cmd.Flags().GetStringSlice("set")
		helmArgs := cmdutil.HelmSetArgs(helmArgRaw)
		devServer, _ := cmd.Flags().GetBool("dev-server")
		noRuntime, _ := cmd.Flags().GetBool("no-runtime")
		wait, _ := cmd.Flags().GetString("wait")
		skipRaw, _ := cmd.Flags().GetStringSlice("skip")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		reportFile, _ := cmd.Flags().GetString("report-file")

		// Discovery mode: -f ./... or -f ./some/path/...
		if strings.HasSuffix(file, "/...") || file == "./..." || file == "..." {
			root := strings.TrimSuffix(file, "/...")
			if root == "." || root == "" {
				root = "."
			}
			if dryRun {
				return dryRunDiscovery(root, skipRaw)
			}
			return runDiscovery(cmd, root, wait, skipRaw, clusterCtx, useCurrentCtx, keepCluster, devServer, workers, version, valuesFiles, helmArgs)
		}

		if dryRun {
			return validate.ValidateE2EFile(file)
		}

		runner, err := e2e.New(file, e2e.Options{
			ClusterCtx: clusterCtx, UseCurrentCtx: useCurrentCtx,
			KeepCluster: keepCluster, Workers: workers,
			DevServer: devServer, InrunVersion: version, InrunChart: chart,
			ValueFiles: valuesFiles, HelmArgs: helmArgs,
			ReportFile: reportFile, NoRuntime: noRuntime,
		})
		if err != nil {
			return err
		}
		_, err = runner.Run(cmd.Context())
		return err
	},
}

// dryRunDiscovery lists the files that would be discovered without running them.
func dryRunDiscovery(root string, skip []string) error {
	var patterns []string
	for _, s := range skip {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				patterns = append(patterns, p)
			}
		}
	}

	paths, err := e2e.DiscoverE2EFiles(root, patterns)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("No e2e files found under %s\n", root)
		return nil
	}

	absRoot, _ := filepath.Abs(root)
	const maxShow = 10
	fmt.Printf("→ Would run %d e2e file(s) under %s\n\n", len(paths), root)
	for i, p := range paths {
		if i >= maxShow {
			fmt.Printf("  ... %d more\n", len(paths)-maxShow)
			break
		}
		rel, _ := filepath.Rel(absRoot, p)
		fmt.Printf("  %s\n", rel)
	}
	return nil
}

// runDiscovery finds all *e2e.yaml leaf files under root, builds a temp
// aggregator, and runs it as a normal suite.
func runDiscovery(cmd *cobra.Command, root, wait string, skip []string, clusterCtx string, useCurrentCtx, keepCluster, devServer bool, workers int, version string, valuesFiles, helmArgs []string) error {
	chart, err := chartFlag(cmd)
	if err != nil {
		return err
	}
	// Expand comma-separated skip entries
	var patterns []string
	for _, s := range skip {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				patterns = append(patterns, p)
			}
		}
	}

	paths, err := e2e.DiscoverE2EFiles(root, patterns)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("No e2e files found under %s\n", root)
		return nil
	}
	absRoot, _ := filepath.Abs(root)
	fmt.Printf("→ Discovered %d e2e file(s) under %s\n", len(paths), root)
	for _, p := range paths {
		rel, _ := filepath.Rel(absRoot, p)
		fmt.Printf("    %s\n", rel)
	}
	fmt.Println()

	suite := e2e.BuildDiscoveryE2E(paths, wait)

	// Write to a temp file so the runner resolves relative paths correctly.
	tmp, err := os.CreateTemp("", "inrun-e2e-discovery-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp suite file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if err := yaml.NewEncoder(tmp).Encode(suite); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp suite: %w", err)
	}
	tmp.Close()

	runner, err := e2e.New(tmp.Name(), e2e.Options{ClusterCtx: clusterCtx, UseCurrentCtx: useCurrentCtx, KeepCluster: keepCluster, Workers: workers, DevServer: devServer, InrunVersion: version, InrunChart: chart, ValueFiles: valuesFiles, HelmArgs: helmArgs})
	if err != nil {
		return err
	}
	_, err = runner.Run(cmd.Context())
	return err
}

// crdInfo holds the name and kind of a CRD entry, used when scaffolding e2e.yaml.
type crdInfo struct {
	name string
	kind string
}

// ── inrun e2e init ─────────────────────────────────────────────────────────────

var e2eInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold an e2e.yaml from the current Blueprint",
	Long: `Reads the Blueprint in the current directory and generates a best-practice
e2e.yaml skeleton: CR created → resources created → cleanup verified.
No cluster is needed — it reads only the Blueprint for CRD names and kinds.

  inrun e2e init                         # auto-detect blueprint.yaml
  inrun e2e init -f blueprint.yaml         # explicit
  inrun e2e init --force                 # overwrite existing e2e.yaml
  inrun e2e init --dry-run               # preview without writing
  inrun e2e init --suite                 # aggregate all e2e.yaml files under .
  inrun e2e init --suite ./examples/     # aggregate under a specific dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if suite, _ := cmd.Flags().GetBool("suite"); suite {
			return e2eInitSuite(cmd, args)
		}
		return e2eInitScaffold(cmd)
	},
}

// e2eInitScaffold reads the Blueprint and writes an e2e.yaml scaffold.
func e2eInitScaffold(cmd *cobra.Command) error {
	blueprintFile, _ := cmd.Flags().GetString("file")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	var err error
	blueprintFile, err = cmdutil.ResolveBlueprintFile(blueprintFile)
	if err != nil {
		return err
	}

	bp, err := blueprint.ParseFile(blueprintFile)
	if err != nil {
		return fmt.Errorf("parsing Blueprint: %w", err)
	}

	outPath := filepath.Join(cmdutil.DirTest, cmdutil.FileE2e)
	if !dryRun && !force {
		if cmdutil.FileExists(outPath) {
			return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
		}
	}

	// Paths in the spec are relative to the spec file, in test/.
	relBlueprint := relFromDir(cmdutil.DirTest, blueprintFile)
	relCRD := relFromDir(cmdutil.DirTest, projectFile(blueprintFile, cmdutil.FileCrd))
	relCR := relFromDir(cmdutil.DirTest, projectFile(blueprintFile, cmdutil.FileCr))

	crdNames := bp.CRDNames()
	var crds []crdInfo
	for _, n := range crdNames {
		entry, ok := bp.CRDEntry(n)
		if !ok {
			continue
		}
		crds = append(crds, crdInfo{name: n, kind: entry.APITypes.Kind})
	}
	sort.Slice(crds, func(i, j int) bool { return crds[i].name < crds[j].name })

	output := buildE2EScaffoldYAML(bp.Metadata().Name, relBlueprint, relCRD, relCR, crds)

	if dryRun {
		fmt.Print(output)
		return nil
	}

	if err := os.MkdirAll(cmdutil.DirTest, 0755); err != nil {
		return fmt.Errorf("creating %s/: %w", cmdutil.DirTest, err)
	}
	if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
	fmt.Printf("  %d CRD(s): %s\n", len(crds), crdKindList(crds))
	fmt.Printf("\n  Edit the placeholders, then run %s.\n", cmdutil.Bold("inrun e2e"))
	return nil
}

// e2eInitSuite discovers all e2e.yaml leaf files and writes a pure aggregator.
func e2eInitSuite(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	wait, _ := cmd.Flags().GetString("wait")
	skipRaw, _ := cmd.Flags().GetStringSlice("skip")

	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	var patterns []string
	for _, s := range skipRaw {
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				patterns = append(patterns, p)
			}
		}
	}

	paths, err := e2e.DiscoverE2EFiles(root, patterns)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("No e2e files found under %s\n", root)
		return nil
	}

	outPath := cmdutil.FileE2e
	if !dryRun && !force {
		if cmdutil.FileExists(outPath) {
			return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
		}
	}

	// Build relative import paths against CWD so the written file is portable.
	cwd, _ := os.Getwd()
	discovered := e2e.BuildDiscoveryE2E(paths, wait)
	type suiteImport struct {
		Path string `yaml:"path"`
		Wait string `yaml:"wait,omitempty"`
	}
	type suiteMeta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description,omitempty"`
	}
	// Pure aggregator — no spec, just imports. Use a local struct so the
	// empty E2ESpec fields are not serialized.
	type suiteDoc struct {
		APIVersion string        `yaml:"apiVersion"`
		Kind       string        `yaml:"kind"`
		Metadata   suiteMeta     `yaml:"metadata"`
		Imports    []suiteImport `yaml:"imports"`
	}
	doc := suiteDoc{
		APIVersion: "inrun.dev/v1",
		Kind:       "E2E",
		Metadata: suiteMeta{
			Name:        "suite",
			Description: fmt.Sprintf("Generated by inrun e2e init --suite — %d file(s) discovered", len(paths)),
		},
	}
	for _, imp := range discovered.Imports {
		rel, err := filepath.Rel(cwd, imp.Path)
		if err != nil {
			rel = imp.Path
		}
		doc.Imports = append(doc.Imports, suiteImport{Path: "./" + rel, Wait: imp.Wait})
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding suite: %w", err)
	}

	output := append([]byte("# Schema reference: "+cmdutil.SchemaRefE2EImports+"\n"), buf.Bytes()...)

	if dryRun {
		fmt.Print(string(output))
		return nil
	}

	if err := os.WriteFile(outPath, output, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	absRoot, _ := filepath.Abs(root)
	fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
	fmt.Printf("  %d file(s) discovered under %s\n", len(paths), absRoot)
	const maxShow = 10
	for i, imp := range doc.Imports {
		if i >= maxShow {
			fmt.Printf("  ... %d more\n", len(doc.Imports)-maxShow)
			break
		}
		fmt.Printf("    %s\n", cmdutil.Dim(imp.Path))
	}
	fmt.Printf("\n  Run %s to verify.\n", cmdutil.Bold("inrun e2e"))
	return nil
}

// buildE2EScaffoldYAML returns the scaffold YAML string for inrun e2e init.
// Comments are injected directly — yaml.Marshal cannot preserve them.
func buildE2EScaffoldYAML(blueprintName, blueprintFile, crdFile, crFile string, crds []crdInfo) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(s + "\n") }

	primaryKind := "<Kind>"
	if len(crds) > 0 {
		primaryKind = crds[0].kind
	}

	w("# Schema reference: " + cmdutil.SchemaRefE2E)
	w("apiVersion: inrun.dev/v1")
	w("kind: E2E")
	w("metadata:")
	w("  name: " + blueprintName + "-e2e")
	w(`  description: "Generated by inrun e2e init — edit to refine"`)
	w("")
	w("spec:")
	w("  blueprint: " + blueprintFile)
	w("  crd: " + crdFile)
	w("  cr: " + crFile)
	w("")
	w("  cluster:")
	w("    provider: kind")
	w("    name: " + blueprintName + "-e2e")
	w("    reuse: false")

	if len(crds) > 1 {
		w("")
		w("  # Multiple CRDs in this Blueprint. Apply the remaining CRDs and CRs via setup.")
		w("  setup:")
		w("    apply:")
		w("      - ./other-crds.yaml   # CRD definitions for the remaining kinds")
		w("      - ./other-crs.yaml    # CR instances for the remaining kinds")
		w("      # Additional CRDs in this Blueprint:")
		for _, c := range crds[1:] {
			w("      # - " + c.kind)
		}
	}

	w("")
	w("  expect:")
	w("    - name: CR created")
	w("      after: cr-applied")
	w("      timeout: 60s")
	w("      resources:")
	w("        - kind: " + primaryKind)
	w("          name: <name>")
	w("          namespace: <namespace>")
	w("")
	w("    # - name: Resources created")
	w("    #   after: cr-applied")
	w("    #   timeout: 60s")
	w("    #   resources:")
	w("    #     - kind: Deployment")
	w("    #       name: <resource-name>")
	w("    #       namespace: <namespace>")
	w("    #       ready: true")
	w("")
	w("    - name: Cleanup verified")
	w("      after: cr-deleted")
	w("      timeout: 30s")
	w("      resources:")
	w("        - kind: " + primaryKind)
	w("          name: <name>")
	w("          namespace: <namespace>")
	w("          count: 0")
	w("        # - kind: Deployment")
	w("        #   name: <resource-name>")
	w("        #   namespace: <namespace>")
	w("        #   count: 0")

	if len(crds) > 1 {
		w("      # Add cleanup assertions for other CRDs as needed.")
	}

	return b.String()
}

// crdKindList returns a comma-separated list of CRD kind names for output.
func crdKindList(crds []crdInfo) string {
	kinds := make([]string, 0, len(crds))
	for _, c := range crds {
		kinds = append(kinds, c.kind)
	}
	return strings.Join(kinds, ", ")
}

func init() {
	cmdutil.RootCmd.AddCommand(e2eCmd)
	e2eCmd.AddCommand(e2eInitCmd)

	e2eInitCmd.Flags().StringP("file", "f", "", "Path to blueprint.yaml or stack.yaml")
	e2eInitCmd.Flags().Bool("force", false, "Overwrite existing e2e.yaml")
	e2eInitCmd.Flags().Bool("dry-run", false, "Print the generated e2e.yaml to stdout instead of writing the file")
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
	e2eCmd.Flags().Bool("dry-run", false, "Print what would run without executing. Single file: runs validate. ./...: lists discovered files.")
	e2eCmd.Flags().String("report-file", "", "Write test results as markdown to this file (e.g. $GITHUB_STEP_SUMMARY). Results are always printed to stdout.")

	// Shadow global flags so they don't appear under `inrun e2e`
	cmdutil.ShadowGlobalCommandFlags(e2eCmd)
}

// chartFlag returns --chart as an absolute chart directory, or "" when unset.
func chartFlag(cmd *cobra.Command) (string, error) {
	chart, _ := cmd.Flags().GetString("chart")
	return cmdutil.ChartDir(chart)
}
