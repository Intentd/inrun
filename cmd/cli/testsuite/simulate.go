//go:build !runtime && !gateway

package testsuite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/validate"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/registry/simulate"
	simulatereport "github.com/intentd/inrun/pkg/registry/simulate/report"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/tools/devserver"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CliSimulateOptions groups the CLI-level flags that flow through all simulate helpers.
type CliSimulateOptions struct {
	CRDName      string
	MaxCycles    int
	Target       string
	SkipExternal bool
	DebugOps     bool
	Verbose      bool // print reconciler logs and each stage of building a CR from spec.intent
	UseEnvtest   bool
	Compare      bool // run in memory and on envtest, and compare the two
	K8sVersion   string
	ReportFile   string
}

// backend is where the flags say a single simulation runs.
func (o CliSimulateOptions) backend() simulate.Backend {
	if o.UseEnvtest {
		return simulate.Envtest
	}
	return simulate.InMemory
}

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Simulate operator reconciliation in memory — no cluster required",
	Long: `Runs the operator reconcile loop against a fake in-memory cluster.
Shows resource creation and state transitions across reconcile cycles.

The recommended entry point is simulate.yaml — it records what your operator
should produce so the run is repeatable and verifiable:

  inrun simulate                              # simulate.yaml auto-detected
  inrun simulate -f simulate.yaml             # explicit — assert mode when expect: is set
  inrun simulate -f blueprint.yaml --cr cr.yaml # direct flags; op-print only
  inrun simulate ./...                        # discovers all simulate.yaml files recursively`,
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		if len(args) > 0 {
			file = args[0]
		}
		if file == "" {
			file = cmdutil.FindFile(cmdutil.FileSimulate)
		}
		if file == "" {
			return fmt.Errorf("no %s found here or in %s/ — pass -f <file>", cmdutil.FileSimulate, cmdutil.DirTest)
		}

		skip, _ := cmd.Flags().GetStringSlice("skip")
		limit, _ := cmd.Flags().GetInt("limit")
		validateOnly, _ := cmd.Flags().GetBool("validate")
		listOnly, _ := cmd.Flags().GetBool("list")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		action, err := pickAction(file, validateOnly, listOnly, dryRun)
		if err != nil {
			return err
		}

		switch action {
		case actionList:
			root, _ := fileio.IsDiscoveryArg(file)
			return listSimulateDiscovery(root, skip, limit)

		case actionValidate:
			if root, ok := fileio.IsDiscoveryArg(file); ok {
				return validateSimulateDiscovery(root, skip)
			}
			return validate.ValidateSimulateFile(file)

		case actionDryRun:
			// Print what would run: the ops for one file, the discovered
			// files for ./...
			if root, ok := fileio.IsDiscoveryArg(file); ok {
				return listSimulateDiscovery(root, skip, limit)
			}
			return validate.ValidateSimulateFile(file)
		}

		simOpts := CliSimulateOptions{}
		simOpts.CRDName, _ = cmd.Flags().GetString("crd")
		simOpts.MaxCycles, _ = cmd.Flags().GetInt("cycles")
		simOpts.Target, _ = cmd.Flags().GetString("target")
		simOpts.SkipExternal, _ = cmd.Flags().GetBool("skip-external")
		simOpts.DebugOps, _ = cmd.Flags().GetBool("debug-ops")
		simOpts.Verbose, _ = cmd.Flags().GetBool("verbose")
		simOpts.UseEnvtest, _ = cmd.Flags().GetBool("envtest")
		simOpts.Compare, _ = cmd.Flags().GetBool("compare")
		if simOpts.Compare && simOpts.UseEnvtest {
			return fmt.Errorf("--compare and --envtest are mutually exclusive: --compare already runs envtest")
		}
		simOpts.K8sVersion, _ = cmd.Flags().GetString("k8s-version")
		simOpts.ReportFile, _ = cmd.Flags().GetString("report")
		if simOpts.ReportFile == "-" {
			simOpts.ReportFile = cmdutil.FileReport
		}

		if devServer, _ := cmd.Flags().GetBool("dev-server"); devServer {
			devServerPort, _ := cmd.Flags().GetInt("dev-server-port")
			if err := devserver.Start(devServerPort); err != nil {
				return fmt.Errorf("starting dev server: %w", err)
			}
		}

		// Every path returns its result, and the report is written from it
		// here: for ./..., one file, or a Blueprint and its CR alike.
		report := func(res *simulate.Result, err error) error {
			if res != nil && simOpts.ReportFile != "" {
				_ = simulatereport.Write(simOpts.ReportFile, res)
			}
			return err
		}

		if root, ok := fileio.IsDiscoveryArg(file); ok {
			return report(runSimulateDiscovery(cmd.Context(), root, skip, simOpts))
		}

		if e2e.IsE2EDoc(file) {
			return errors.Join(
				fmt.Errorf("%s is an E2E file, not a Simulate file", file),
				fmt.Errorf("run it on a cluster:     inrun e2e -f %s", file),
				fmt.Errorf("or generate a Simulate:  inrun simulate init"),
			)
		}

		// Only a Simulate file is validated as one: a Blueprint (the -f
		// blueprint.yaml --cr form) would fail for lacking spec.blueprint.
		if simulate.IsSimulateDoc(file) {
			if err := validate.ValidateSimulateFileQuiet(file); err != nil {
				return err
			}
			return report(RunSimulateFromSpec(cmd.Context(), file, simOpts))
		}

		crFile, _ := cmd.Flags().GetString("cr")
		if crFile == "" {
			crFile = cmdutil.FindFile(cmdutil.FileCr)
		}
		if crFile == "" {
			return fmt.Errorf("--cr is required")
		}

		return report(RunSimulate(cmd.Context(), file, crFile, simOpts))
	},
}

// RunSimulate simulates a Blueprint's CRDs against the CRs in crFile, with
// no expectations: a run passes unless a cycle returns an error. It returns
// the CRDs' results folded into one, for the report; on an error, the
// result holds the CRDs simulated before it.
func RunSimulate(ctx context.Context, blueprintFile, crFile string, simOpts CliSimulateOptions) (*simulate.Result, error) {
	start := time.Now()
	maxCycles := simOpts.MaxCycles
	if maxCycles <= 0 {
		maxCycles = 10
	}

	m := merger.New(blueprintFile)
	if err := m.Merge(); err != nil {
		return nil, fmt.Errorf("merging Blueprint: %w", err)
	}
	crdPaths := BlueprintCRDPaths(m)
	bp, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		var typedErr *blueprint.TypedOperatorError
		if errors.As(err, &typedErr) {
			cmdutil.PrintTypedOperatorHint(typedErr, "inrun simulate")
		}
		return nil, fmt.Errorf("parsing Blueprint: %w", err)
	}

	crData, err := cmdutil.ReadLocal(crFile)
	if err != nil {
		return nil, fmt.Errorf("reading CR: %w", err)
	}

	// Parse all documents; key by lowercase kind so each CRD gets its own CR.
	crs, err := cmdutil.ParseMultiDocCRs(crData)
	if err != nil {
		return nil, err
	}
	if len(crs) == 0 {
		return nil, fmt.Errorf("reading CR: no valid documents found in %s", crFile)
	}

	// If --crd is given, simulate that CRD only. Otherwise simulate all.
	var targets []string
	if simOpts.CRDName != "" {
		targets = []string{simOpts.CRDName}
	} else {
		targets = bp.CRDNames()
	}

	opts := simulate.RunOptions{SkipExternal: simOpts.SkipExternal, Target: simOpts.Target, Verbose: simOpts.Verbose}
	agg := &simulate.Result{Name: filepath.Base(blueprintFile)}
	done := func(err error) (*simulate.Result, error) {
		agg.Elapsed = time.Since(start)
		agg.Assertions.Status = agg.SimulateStatusFor()
		return agg, err
	}

	for _, name := range targets {
		crdEntry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
		if !ok {
			if len(targets) > 1 {
				fmt.Printf("  %s\n\n", cmdutil.Dim(fmt.Sprintf("note: no CR found for %s — skipped", crdEntry.APITypes.Kind)))
				continue
			}
			return done(fmt.Errorf("no CR found for CRD %q (kind: %s) in %s", name, crdEntry.APITypes.Kind, crFile))
		}
		opts.Blueprint = bp
		opts.CRDName = name
		opts.CR = in.CR
		opts.MaxCycles = maxCycles
		opts.Peers = in.Peers
		opts.ExistingInstances = in.Existing
		res, err := SimulateOne(ctx, opts, simOpts, crdPaths, nil, template.NewResolverWithFunc(bp.UserFunctions()))
		agg.Add(res)
		if err != nil {
			return done(err)
		}
	}
	return done(nil)
}

// SimulateOne simulates one CR and prints the run: its cycles, its steady
// state and, with expect, its assertions. With --compare it runs both
// backends and compares them instead (compareOne). The simulate package
// runs and judges; this only sequences that and prints it.
func SimulateOne(
	ctx context.Context,
	opts simulate.RunOptions,
	simOpts CliSimulateOptions,
	crdPaths []string,
	expect *types.SimulateExpect,
	resolver *template.Resolver,
) (*simulate.Result, error) {
	if simOpts.Compare {
		return compareOne(ctx, opts, simOpts, crdPaths, expect, resolver)
	}
	if opts.MaxCycles == 0 {
		opts.MaxCycles = 10
	}

	printSimulateHeader(opts, simOpts, simOpts.backend().String())

	opts.CRDPaths = crdPaths
	opts.K8sVersion = simOpts.K8sVersion
	result, err := underSpinner(fmt.Sprintf("Running %d cycles...", opts.MaxCycles), simOpts.Verbose, func() (*simulate.Result, error) {
		return simulate.New(ctx, opts).Execute(simOpts.backend())
	})
	if err != nil {
		fmt.Printf("\n  %s %v\n", cmdutil.Red("error:"), err)
		return nil, err
	}

	printRun(result, opts.MaxCycles, simOpts.DebugOps)

	errs, err := simulate.Evaluate(result, expect, resolver, opts.CR)
	if expect != nil {
		printAssertions(errs, expect)
	}
	return result, err
}

// RunSimulateFromSpec loads a simulate.yaml and runs it in assert mode.
// Aggregator form (imports, no spec) expands each imported file in order and
// aggregates their results.
func RunSimulateFromSpec(ctx context.Context, path string, simOpts CliSimulateOptions) (*simulate.Result, error) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var doc types.Simulate
	if err := cmdutil.StrictUnmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s:\n%s", path, err)
	}

	dir := filepath.Dir(path)
	start := time.Now()

	// Aggregator: imports but no spec.
	if doc.Spec == nil && len(doc.Imports) > 0 {
		agg := &simulate.Result{Name: doc.Metadata.Name}
		var failed []string
		for _, imp := range doc.Imports {
			impPath := imp
			if !filepath.IsAbs(impPath) {
				impPath = filepath.Join(dir, impPath)
			}
			res, err := RunSimulateFromSpec(ctx, impPath, simOpts)
			agg.AddImport(imp, res, err)
			if err != nil {
				failed = append(failed, imp+": "+err.Error())
			}
		}
		agg.Elapsed = time.Since(start)
		agg.Assertions.Status = agg.SimulateStatusFor()
		if len(failed) > 0 {
			return agg, fmt.Errorf("simulate failed for: %s", strings.Join(failed, ", "))
		}
		return agg, nil
	}

	if doc.Spec == nil {
		return nil, fmt.Errorf("%s: missing spec", path)
	}

	if err := validate.ValidateSimulateFileQuiet(path); err != nil {
		return nil, err
	}

	if err := types.ExpandSimulateOpsAndAbsentIncludes(doc.Spec.Expect, dir); err != nil {
		return nil, fmt.Errorf("expanding simulate ops includes in %s: %w", path, err)
	}

	cycles := doc.Spec.Cycles
	if cycles <= 0 {
		cycles = simOpts.MaxCycles
	}

	// CLI flag wins over spec field for both target and skipExternal.
	effectiveTarget := simOpts.Target
	if effectiveTarget == "" {
		effectiveTarget = doc.Spec.Target
	}
	opts := simulate.RunOptions{
		Verbose:      simOpts.Verbose,
		SkipExternal: simOpts.SkipExternal || doc.Spec.SkipExternal,
		Target:       effectiveTarget,
	}

	blueprintPath := filepath.Join(dir, doc.Spec.Blueprint)

	// Resolve CRD paths (for --envtest) relative to the simulate.yaml directory.
	crdPaths := make([]string, 0, len(doc.Spec.AllCRDPaths()))
	for _, p := range doc.Spec.AllCRDPaths() {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, abs)
		}
		crdPaths = append(crdPaths, abs)
	}

	for _, p := range doc.Spec.SetupPaths() {
		opts.Setup = append(opts.Setup, cmdutil.JoinPath(path, p))
	}

	m := merger.New(blueprintPath)
	if err := m.Merge(); err != nil {
		return nil, fmt.Errorf("merging Blueprint: %w", err)
	}
	if len(crdPaths) == 0 {
		crdPaths = BlueprintCRDPaths(m)
	}

	bp, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		var typedErr *blueprint.TypedOperatorError
		if errors.As(err, &typedErr) {
			cmdutil.PrintTypedOperatorHint(typedErr, "inrun simulate")
		}
		return nil, fmt.Errorf("parsing Blueprint: %w", err)
	}

	functions := doc.Spec.Functions
	if merged, err := functions.Merge(bp.UserFunctions(), "blueprint"); err == nil {
		functions = merged
	}
	resolver := template.NewResolverWithFunc(functions)

	var crs map[string][]*unstructured.Unstructured
	if doc.Spec.Intent != "" {
		// The CR comes from the intent, built as the gateway would.
		if PlayIntent == nil {
			return nil, fmt.Errorf("%s: spec.intent is not supported in this build", path)
		}
		obj, err := PlayIntent(PlayRequest{
			Blueprint:     bp,
			BlueprintFile: blueprintPath,
			IntentFile:    filepath.Join(dir, doc.Spec.Intent),
			Token:         doc.Spec.Token,
			Op:            types.ServeOpCreate,
			Cycles:        doc.Spec.Cycles,
			Verbose:       simOpts.Verbose,
		})
		if err != nil {
			return nil, fmt.Errorf("playing %s: %w", doc.Spec.Intent, err)
		}

		crs = map[string][]*unstructured.Unstructured{cmdutil.CRKey(obj.GetKind()): {obj}}
	} else {
		var err error
		if crs, err = readSpecCRs(dir, doc.Spec.AllCRPaths()); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}

	var targets []string
	if simOpts.CRDName != "" {
		targets = []string{simOpts.CRDName}
	} else {
		targets = bp.CRDNames()
	}

	if err := simulate.CheckExpectCRDs(doc.Spec.Expect, bp.CRDNames()); err != nil {
		return nil, err
	}

	agg := &simulate.Result{Name: doc.Metadata.Name}
	var failed []string
	for _, name := range targets {
		crdEntry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
		if !ok {
			if len(targets) > 1 {
				continue
			}
			return nil, fmt.Errorf("no CR found for CRD %q (kind: %s)", name, crdEntry.APITypes.Kind)
		}
		opts.Blueprint = bp
		opts.CRDName = name
		opts.CR = in.CR
		opts.MaxCycles = cycles
		opts.Peers = in.Peers
		opts.ExistingInstances = in.Existing
		expect := doc.Spec.ExpectForCRD(name)
		res, err := SimulateOne(ctx, opts, simOpts, crdPaths, expect, resolver)
		if err != nil {
			failed = append(failed, name+": "+err.Error())
		}
		agg.Add(res)
	}

	agg.Elapsed = time.Since(start)
	agg.Assertions.Status = agg.SimulateStatusFor()
	if len(failed) > 0 {
		return agg, fmt.Errorf("simulate failed for: %s", strings.Join(failed, ", "))
	}
	return agg, nil
}

// ── Discovery mode ─────────────────────────────────────────────────────────────

type simulateFileResult struct {
	path      string
	skipped   bool
	skipMsg   string
	steady    bool
	cycle     int
	elapsed   time.Duration
	cycleErrs bool
}

func init() {
	cmdutil.RootCmd.AddCommand(simulateCmd)
	simulateCmd.AddCommand(simulateInitCmd)

	simulateInitCmd.Flags().StringP("file", "f", "", "Path to blueprint.yaml or stack.yaml")
	simulateInitCmd.Flags().String("cr", "", "Path to the CR YAML file")
	simulateInitCmd.Flags().Bool("force", false, "Overwrite existing simulate.yaml")
	simulateInitCmd.Flags().Bool("dry-run", false, "Print the generated simulate.yaml to stdout instead of writing the file")
	simulateInitCmd.Flags().Bool("suite", false, "Aggregate all simulate.yaml leaf files found under the given dir (default: .)")
	simulateInitCmd.Flags().StringSlice("skip", []string{}, "Comma-separated path patterns to exclude from suite discovery")

	simulateCmd.Flags().StringP("file", "f", "", "Path to blueprint.yaml")
	simulateCmd.Flags().String("cr", "", "Path to the CR YAML file to simulate")
	simulateCmd.Flags().String("crd", "", "CRD name to simulate (default: all CRDs in Blueprint)")
	simulateCmd.Flags().Int("cycles", 10, "Maximum number of reconcile cycles")
	simulateCmd.Flags().StringP("target", "t", "", "Target that provides the operatorbox for simulate")
	simulateCmd.Flags().StringSlice("skip", []string{}, "Comma-separated path patterns to skip during ./... discovery (e.g. vendor,testdata)")
	simulateCmd.Flags().Bool("skip-external", false, "Stub external: HTTP calls with empty 200 responses instead of hitting the real network")
	simulateCmd.Flags().Bool("debug-ops", false, "Print every recorded op with its cycle number (diagnostic)")
	simulateCmd.Flags().Bool("verbose", false, "Print the reconciler's logs, and each stage of building a CR from spec.intent")
	simulateCmd.Flags().Bool("dev-server", false, "Start the mock dev server for external: examples")
	simulateCmd.Flags().Int("dev-server-port", devserver.Port, "Port for the mock dev server")
	simulateCmd.Flags().Bool("envtest", false, "Run against a local kube-apiserver + etcd instead of fake clients (binaries auto-downloaded to ~/.inrun/envtest-bins on first use)")
	simulateCmd.Flags().BoolP("compare", "c", false, "Run in memory and on envtest from the same input, and compare their writes and final objects; fails on any difference")
	simulateCmd.Flags().String("k8s-version", simulate.DefaultEnvtestK8sVersion, "Kubernetes version for envtest binaries (e.g. 1.31, 1.32); only used with --envtest")
	simulateCmd.Flags().Bool("validate", false, "Validate the spec without running it. Single file: validate it. ./...: validate every discovered file.")
	simulateCmd.Flags().BoolP("list", "l", false, "List the files ./... would run, without running them.")
	simulateCmd.Flags().Bool("dry-run", false, "Print what would run without executing. Single file: runs validate. ./...: lists discovered files.")
	simulateCmd.Flags().Int("limit", 10, "maximum number of discovered simulate files to list with --list or --dry-run (0 = no limit)")
	simulateCmd.Flags().StringP("report", "r", "", "Write results as Markdown or JSON based on the file extension. Results are always printed to stdout.")
	simulateCmd.Flags().Lookup("report").NoOptDefVal = "-"

	// Shadow global flags so they don't appear under `inrun simulate`
	cmdutil.ShadowGlobalCommandFlags(simulateCmd)
}
