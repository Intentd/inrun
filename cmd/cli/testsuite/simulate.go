//go:build !runtime && !gateway

package testsuite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/util/term"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/validate"

	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/tools/devserver"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	K8sVersion   string
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
  inrun simulate -f catalog.yaml --cr cr.yaml # direct flags; op-print only
  inrun simulate ./...                        # discovers all simulate.yaml files recursively`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cliOpts := CliSimulateOptions{}
		cliOpts.CRDName, _ = cmd.Flags().GetString("crd")
		cliOpts.MaxCycles, _ = cmd.Flags().GetInt("cycles")
		cliOpts.Target, _ = cmd.Flags().GetString("target")
		cliOpts.SkipExternal, _ = cmd.Flags().GetBool("skip-external")
		cliOpts.DebugOps, _ = cmd.Flags().GetBool("debug-ops")
		cliOpts.Verbose, _ = cmd.Flags().GetBool("verbose")
		cliOpts.UseEnvtest, _ = cmd.Flags().GetBool("envtest")
		cliOpts.K8sVersion, _ = cmd.Flags().GetString("k8s-version")

		if devServer, _ := cmd.Flags().GetBool("dev-server"); devServer {
			devServerPort, _ := cmd.Flags().GetInt("dev-server-port")
			if err := devserver.Start(devServerPort); err != nil {
				return fmt.Errorf("starting dev server: %w", err)
			}
		}

		// Discovery mode: inrun simulate ./...
		if len(args) > 0 && args[0] == "./..." {
			skipRaw, _ := cmd.Flags().GetStringSlice("skip")
			return runSimulateDiscovery(cmd.Context(), ".", skipRaw, cliOpts)
		}

		catalogFile, _ := cmd.Flags().GetString("file")
		if catalogFile == "" {
			// Auto-detect: simulate.yaml (here, then test/) → catalog.yaml/stack.yaml
			switch sim := cmdutil.FindFile(cmdutil.FileSimulate, cmdutil.DirTest); {
			case sim != "":
				catalogFile = sim
			default:
				if d := cmdutil.DefaultFilePaths(); len(d) > 0 {
					catalogFile = d[0]
				}
			}
		}
		if catalogFile == "" {
			return fmt.Errorf(cmdutil.ErrNoCatalog)
		}

		// Simulate kind: assert mode
		if simulate.IsSimulateDoc(catalogFile) {
			return RunSimulateFromSpec(cmd.Context(), catalogFile, cliOpts)
		}

		// Reject E2E files with a clear message
		if e2e.IsE2EDoc(catalogFile) {
			return fmt.Errorf("%s is an E2E file — use 'inrun e2e' for cluster testing, or run 'inrun simulate init' to generate a simulate.yaml", catalogFile)
		}

		crFile, _ := cmd.Flags().GetString("cr")
		if crFile == "" {
			crFile = cmdutil.FindFile(cmdutil.FileCr, cmdutil.DirManifests)
		}
		if crFile == "" {
			return fmt.Errorf("--cr is required")
		}

		return RunSimulate(cmd.Context(), catalogFile, crFile, cliOpts)
	},
}

func RunSimulate(ctx context.Context, catalogFile, crFile string, cliOpts CliSimulateOptions) error {
	maxCycles := cliOpts.MaxCycles
	if maxCycles <= 0 {
		maxCycles = 10
	}

	m := merger.New(catalogFile)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging Catalog: %w", err)
	}
	crdPaths := CatalogCRDPaths(m)
	cat, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		var typedErr *catalog.TypedOperatorError
		if errors.As(err, &typedErr) {
			cmdutil.PrintTypedOperatorHint(typedErr, "inrun simulate")
		}
		return fmt.Errorf("parsing Catalog: %w", err)
	}

	crData, err := cmdutil.ReadLocal(crFile)
	if err != nil {
		return fmt.Errorf("reading CR: %w", err)
	}

	// Parse all documents; key by lowercase kind so each CRD gets its own CR.
	crs := cmdutil.ParseMultiDocCRs(crData)
	if len(crs) == 0 {
		return fmt.Errorf("reading CR: no valid documents found in %s", crFile)
	}

	// If --crd is given, simulate that CRD only. Otherwise simulate all.
	var targets []string
	if cliOpts.CRDName != "" {
		targets = []string{cliOpts.CRDName}
	} else {
		targets = cat.CRDNames()
	}

	baseOpts := simulate.RunOptions{SkipExternal: cliOpts.SkipExternal, Target: cliOpts.Target}

	for _, name := range targets {
		crdEntry, ok := cat.CRDEntry(name)
		if !ok {
			continue
		}
		in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
		if !ok {
			if len(targets) > 1 {
				fmt.Printf("  %s\n\n", cmdutil.Dim(fmt.Sprintf("note: no CR found for %s — skipped", crdEntry.APITypes.Kind)))
				continue
			}
			return fmt.Errorf("no CR found for CRD %q (kind: %s) in %s", name, crdEntry.APITypes.Kind, crFile)
		}
		crdOpts := baseOpts
		crdOpts.Peers = in.Peers
		crdOpts.ExistingInstances = in.Existing
		if err := SimulateOne(ctx, cat, name, in.CR, maxCycles, crdOpts, cliOpts, crdPaths, nil); err != nil {
			return err
		}
	}
	return nil
}

func SimulateOne(ctx context.Context, cat *catalog.Catalog, crdName string, cr *unstructured.Unstructured, maxCycles int, opts simulate.RunOptions, cliOpts CliSimulateOptions, crdPaths []string, expect *types.SimulateExpect) error {
	if cliOpts.UseEnvtest {
		version := cliOpts.K8sVersion
		if version == "" {
			version = simulate.DefaultEnvtestK8sVersion
		}
		fmt.Printf("Simulating %s/%s   %s\n", crdName, cr.GetName(), cmdutil.Gray("(envtest · kube-apiserver "+version+")"))
	} else {
		fmt.Printf("Simulating %s/%s\n", crdName, cr.GetName())
	}

	// Emit notes for operatorBox blocks that cannot execute in the fake cluster.
	crdEntry, _ := cat.CRDEntry(crdName)
	if crdEntry.Box().EffectiveOnReconcile() != nil && len(crdEntry.Box().EffectiveOnReconcile().External) > 0 {
		if opts.SkipExternal {
			fmt.Printf("  %s\n", cmdutil.Dim("note: external: calls stubbed — result fields will be empty"))
		} else {
			fmt.Printf("  %s\n", cmdutil.Dim("note: external: calls will hit the real network (pass --skip-external to stub)"))
		}
	}
	if len(crdEntry.Box().EffectiveCross()) > 0 && len(opts.Peers) == 0 {
		fmt.Printf("  %s\n", cmdutil.Dim("note: cross: peer CRs not provided — cross.* fields will be empty (add sibling CRs to the CR file)"))
	}
	printSimulateAutoscaleSummary(crdEntry)
	fmt.Println()

	// The reconciler's own logs show only with --verbose; errors always do.
	// With --verbose there is no spinner, so log lines don't break into it.
	var spin *term.Spinner
	if cliOpts.Verbose {
		fmt.Printf("Running %d cycles...\n", maxCycles)
	} else {
		defer logger.Quiet()()
		spin = cmdutil.StartSpinner(fmt.Sprintf("Running %d cycles...", maxCycles))
	}
	start := time.Now()
	var result *simulate.Result
	var err error
	if cliOpts.UseEnvtest {
		if len(crdPaths) == 0 {
			return fmt.Errorf("%s --envtest requires spec.crd or spec.crdFiles to be set", cmdutil.FailureMark())
		}
		result, err = simulate.RunWithEnvtest(ctx, cat, crdName, cr, maxCycles, opts, crdPaths, cliOpts.K8sVersion)
	} else {
		result, err = simulate.Run(ctx, cat, crdName, cr, maxCycles, opts)
	}
	if err != nil {
		if spin != nil {
			spin.Failure()
		}
		fmt.Printf("\n  %s %v\n", cmdutil.Red("error:"), err)
		return err
	}
	if spin != nil {
		spin.Stop()
	}
	elapsed := time.Since(start)

	if cliOpts.DebugOps {
		fmt.Printf("  [debug-ops] %d total ops recorded across all cycles:\n", len(result.AllOps))
		for _, op := range result.AllOps {
			fmt.Printf("  [debug-ops]   cycle=%-2d  verb=%-8s  resource=%-20s  name=%s\n",
				op.Cycle, op.Verb, op.Resource, op.Name)
		}
		fmt.Println()
	}

	for _, note := range result.Notes {
		fmt.Printf("  %s\n", cmdutil.Dim("note: "+note))
	}
	if len(result.Notes) > 0 {
		fmt.Println()
	}

	var prevKey string
	var repeatStart int
	flush := func(upTo int) {
		if repeatStart > 0 && upTo > repeatStart {
			fmt.Printf("  %s\n", cmdutil.Gray(fmt.Sprintf("(cycles %d–%d: identical)", repeatStart, upTo)))
			repeatStart = 0
		}
	}

	for _, cycle := range result.Cycles {
		meaningful := filterOps(cycle.Ops, "create", "update", "delete", "patch", "apply")
		if len(meaningful) == 0 && cycle.Error == nil {
			continue
		}

		key := opsKey(meaningful)
		if key == prevKey && cycle.Error == nil {
			if repeatStart == 0 {
				repeatStart = cycle.Cycle
			}
			continue
		}
		flush(cycle.Cycle - 1)
		prevKey = key

		fmt.Printf("  Cycle %d:\n", cycle.Cycle)
		printCycleOps(meaningful)
		if cycle.Error != nil {
			fmt.Printf("    %s %v\n", cmdutil.FailureMark(), cycle.Error)
		}
	}
	flush(result.Cycles[len(result.Cycles)-1].Cycle)

	if result.Steady {
		fmt.Printf("\n  %s Steady state at cycle %d in %s\n\n", cmdutil.SuccessMark(), result.SteadyAt, elapsed.Round(time.Millisecond))
	} else {
		fmt.Printf("\n  ~ Max cycles reached (%d) in %s\n\n", maxCycles, elapsed.Round(time.Millisecond))
	}

	for _, c := range result.Cycles {
		if c.Error != nil {
			return fmt.Errorf("simulation completed with cycle errors")
		}
	}

	if expect != nil {
		errs := simulate.Assert(result, expect)
		printAssertions(errs, expect)
		if len(errs) > 0 {
			return fmt.Errorf("assertions failed (%d/%d passed)", len(expect.Ops)+boolInt(expect.Steady != nil)+boolInt(expect.NoErrors)-len(errs), len(expect.Ops)+boolInt(expect.Steady != nil)+boolInt(expect.NoErrors))
		}
	}
	return nil
}

// RunSimulateFromSpec loads a simulate.yaml and runs it in assert mode.
// Aggregator form (imports, no spec) expands each imported file in order.
func RunSimulateFromSpec(ctx context.Context, path string, cliOpts CliSimulateOptions) error {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var doc types.Simulate
	if err := cmdutil.StrictUnmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing %s:\n%s", path, err)
	}

	dir := filepath.Dir(path)

	// Aggregator: imports but no spec.
	if doc.Spec == nil && len(doc.Imports) > 0 {
		for _, imp := range doc.Imports {
			impPath := imp
			if !filepath.IsAbs(impPath) {
				impPath = filepath.Join(dir, impPath)
			}
			if err := RunSimulateFromSpec(ctx, impPath, cliOpts); err != nil {
				return err
			}
		}
		return nil
	}

	if doc.Spec == nil {
		return fmt.Errorf("%s: missing spec", path)
	}

	if err := validate.ValidateSimulateFileQuiet(path); err != nil {
		return err
	}

	if err := types.ExpandSimulateOpsIncludes(doc.Spec.Expect, dir); err != nil {
		return fmt.Errorf("expanding simulate ops includes in %s: %w", path, err)
	}

	cycles := doc.Spec.Cycles
	if cycles <= 0 {
		cycles = cliOpts.MaxCycles
	}

	// CLI flag wins over spec field for both target and skipExternal.
	effectiveTarget := cliOpts.Target
	if effectiveTarget == "" {
		effectiveTarget = doc.Spec.Target
	}
	opts := simulate.RunOptions{
		SkipExternal: cliOpts.SkipExternal || doc.Spec.SkipExternal,
		Target:       effectiveTarget,
	}

	catalogPath := filepath.Join(dir, doc.Spec.Catalog)

	// Resolve CRD paths (for --envtest) relative to the simulate.yaml directory.
	crdPaths := make([]string, 0, len(doc.Spec.AllCRDPaths()))
	for _, p := range doc.Spec.AllCRDPaths() {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, abs)
		}
		crdPaths = append(crdPaths, abs)
	}

	m := merger.New(catalogPath)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging Catalog: %w", err)
	}
	if len(crdPaths) == 0 {
		crdPaths = CatalogCRDPaths(m)
	}

	cat, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
	if err != nil {
		var typedErr *catalog.TypedOperatorError
		if errors.As(err, &typedErr) {
			cmdutil.PrintTypedOperatorHint(typedErr, "inrun simulate")
		}
		return fmt.Errorf("parsing Catalog: %w", err)
	}

	var crs map[string][]*unstructured.Unstructured
	if doc.Spec.Intent != "" {
		// The CR comes from the intent, built as the gateway would.
		if PlayIntent == nil {
			return fmt.Errorf("%s: spec.intent is not supported in this build", path)
		}
		obj, err := PlayIntent(PlayRequest{
			Catalog:     cat,
			CatalogFile: catalogPath,
			IntentFile:  filepath.Join(dir, doc.Spec.Intent),
			Token:       doc.Spec.Token,
			Op:          types.ServeOpCreate,
			Cycles:      doc.Spec.Cycles,
			Verbose:     cliOpts.Verbose,
		})
		if err != nil {
			return fmt.Errorf("playing %s: %w", doc.Spec.Intent, err)
		}

		crs = map[string][]*unstructured.Unstructured{cmdutil.CRKey(obj.GetKind()): {obj}}
	} else {
		var err error
		if crs, err = readSpecCRs(dir, doc.Spec.AllCRPaths()); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}

	var targets []string
	if cliOpts.CRDName != "" {
		targets = []string{cliOpts.CRDName}
	} else {
		targets = cat.CRDNames()
	}

	var failed []string
	for _, name := range targets {
		crdEntry, ok := cat.CRDEntry(name)
		if !ok {
			continue
		}
		in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
		if !ok {
			if len(targets) > 1 {
				continue
			}
			return fmt.Errorf("no CR found for CRD %q (kind: %s)", name, crdEntry.APITypes.Kind)
		}
		crdOpts := opts
		crdOpts.Peers = in.Peers
		crdOpts.ExistingInstances = in.Existing
		expect := simulate.ExpectForCRD(doc.Spec.Expect, name)
		if err := SimulateOne(ctx, cat, name, in.CR, cycles, crdOpts, cliOpts, crdPaths, expect); err != nil {
			failed = append(failed, name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("simulate failed for: %s", strings.Join(failed, ", "))
	}
	return nil
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

// runSimulateDiscovery finds all simulate.yaml files under root, simulates each,
// and prints an aggregate summary.
func runSimulateDiscovery(ctx context.Context, root string, skip []string, cliOpts CliSimulateOptions) error {
	var patterns []string
	for _, s := range skip {
		patterns = append(patterns, s)
	}
	paths, err := e2e.DiscoverSimulateFiles(root, patterns)
	if err != nil {
		return fmt.Errorf("discovering simulate files: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("no simulate.yaml files found under %s\n", root)
		return nil
	}

	fmt.Printf("Simulating %d file(s) under %s\n\n", len(paths), root)

	absRoot, _ := filepath.Abs(root)

	// In discovery mode each file declares its own target; don't let a CLI
	// --target flag override every file in the suite.
	fileOpts := cliOpts
	fileOpts.Target = ""

	var results []simulateFileResult
	for _, p := range paths {
		rel, _ := filepath.Rel(absRoot, p)

		start := time.Now()
		err := RunSimulateFromSpec(ctx, p, fileOpts)
		elapsed := time.Since(start)

		var res simulateFileResult
		res.path = rel
		res.elapsed = elapsed
		if err != nil {
			fmt.Printf("  %-55s %s  %s\n", rel, cmdutil.Red("✗ "+err.Error()), cmdutil.Dim("[assert]"))
			res.cycleErrs = true
		} else {
			fmt.Printf("  %-55s %s  %s\n", rel, cmdutil.Green(fmt.Sprintf("✓ passed (%s)", elapsed.Round(time.Millisecond))), cmdutil.Dim("[assert]"))
			res.steady = true
		}
		results = append(results, res)
	}

	// Aggregate summary
	var simulated, skipped int
	var slowest simulateFileResult
	for _, r := range results {
		if r.skipped {
			skipped++
		} else {
			simulated++
			if r.elapsed > slowest.elapsed {
				slowest = r
			}
		}
	}
	fmt.Printf("\n  %d file(s) — %d simulated, %d skipped\n", len(results), simulated, skipped)
	if slowest.path != "" {
		fmt.Printf("  Slowest: %s (%s)\n", slowest.path, slowest.elapsed.Round(time.Millisecond))
	}

	var errFiles []simulateFileResult
	for _, r := range results {
		if r.cycleErrs {
			errFiles = append(errFiles, r)
		}
	}
	if len(errFiles) > 0 {
		fmt.Printf("\n  %s — run directly for full output:\n", cmdutil.Yellow("Files with errors"))
		for _, r := range errFiles {
			fmt.Printf("    inrun simulate -f %s\n", r.path)
		}
		return fmt.Errorf("simulation failed in %d file(s)", len(errFiles))
	}

	return nil
}

// ── inrun simulate init ──────────────────────────────────────────────────────────

type crdOps struct {
	name string
	ops  []simulate.Op
}

var simulateInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a simulate.yaml pre-filled with the observed cycle-1 ops",
	Long: `Runs the reconciler once and generates a simulate.yaml with the
observed cycle-1 create operations as expect: rules. Edit and refine from there.

  inrun simulate init
  inrun simulate init -f catalog.yaml --cr cr.yaml
  inrun simulate init --force              # overwrite existing simulate.yaml
  inrun simulate init --suite              # aggregate all simulate.yaml files under .
  inrun simulate init --suite ./examples/  # aggregate under a specific dir`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if suite, _ := cmd.Flags().GetBool("suite"); suite {
			return simulateInitSuite(cmd, args)
		}

		catalogFile, _ := cmd.Flags().GetString("file")
		crFile, _ := cmd.Flags().GetString("cr")
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		var err error
		catalogFile, err = cmdutil.ResolveCatalogFile(catalogFile)
		if err != nil {
			return err
		}
		if crFile == "" {
			crFile = projectFile(catalogFile, cmdutil.FileCr, cmdutil.DirManifests)
		} else if abs, err := filepath.Abs(crFile); err == nil {
			crFile = abs
		}

		cat, err := catalog.ParseFile(catalogFile)
		if err != nil {
			return fmt.Errorf("parsing Catalog: %w", err)
		}
		crData, err := cmdutil.ReadLocal(crFile)
		if err != nil {
			return fmt.Errorf("reading CR: %w", err)
		}
		crs := cmdutil.ParseMultiDocCRs(crData)
		if len(crs) == 0 {
			return fmt.Errorf("no valid CR documents in %s", crFile)
		}

		outPath := filepath.Join(cmdutil.DirTest, cmdutil.FileSimulate)
		if !dryRun && !force {
			if cmdutil.FileExists(outPath) {
				return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
			}
		}

		opts := simulate.RunOptions{}
		targets := cat.CRDNames()

		var results []crdOps

		for _, name := range targets {
			crdEntry, ok := cat.CRDEntry(name)
			if !ok {
				continue
			}
			in, ok := cmdutil.ResolveCRInputs(crs, crdEntry.APITypes.Kind)
			if !ok {
				continue
			}
			crdOpts := opts
			crdOpts.Peers = in.Peers
			crdOpts.ExistingInstances = in.Existing
			result, err := simulate.Run(cmd.Context(), cat, name, in.CR, 10, crdOpts)
			if err != nil && crdEntry.WithRemoteDecl() {
				fmt.Printf("  %s %s: remote reconciler unreachable — start it and re-run (%v)\n", cmdutil.Dim("note:"), name, err)
				continue
			}
			if err != nil {
				return fmt.Errorf("simulating %s: %w", name, err)
			}
			var cycle1Creates []simulate.Op
			for _, op := range result.AllOps {
				if op.Cycle == 1 && op.Verb == "create" && op.Resource != "namespaces" {
					cycle1Creates = append(cycle1Creates, op)
				}
			}
			results = append(results, crdOps{name: name, ops: cycle1Creates})
		}

		// Paths in the spec are relative to the spec file, so the file is portable.
		relCatalog := relFromDir(cmdutil.DirTest, catalogFile)
		relCR := relFromDir(cmdutil.DirTest, crFile)

		doc := generateSimulateDoc(relCatalog, relCR, cat.Metadata().Name, results)

		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(doc); err != nil {
			return fmt.Errorf("encoding simulate.yaml: %w", err)
		}

		output := injectAbsentComment(buf.Bytes(), results)
		output = append([]byte("# Schema reference: "+cmdutil.SchemaRefSimulate+"\n"), output...)

		if dryRun {
			fmt.Print(string(output))
			return nil
		}

		if err := os.MkdirAll(cmdutil.DirTest, 0755); err != nil {
			return fmt.Errorf("creating %s/: %w", cmdutil.DirTest, err)
		}
		if err := os.WriteFile(outPath, output, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}

		fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
		fmt.Printf("  %d CRD(s), %d op rule(s)\n", len(results), countRules(results))
		fmt.Printf("\n  Run %s to verify.\n", cmdutil.Bold("inrun simulate"))
		return nil
	},
}

// simulateInitSuite discovers all simulate.yaml leaf files under root, builds a
// pure aggregator, and writes (or prints) simulate.yaml in the current directory.
func simulateInitSuite(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
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

	paths, err := e2e.DiscoverSimulateFiles(root, patterns)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	if len(paths) == 0 {
		fmt.Printf("No simulate.yaml files found under %s\n", root)
		return nil
	}

	cwd, _ := os.Getwd()
	relPaths := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(cwd, p)
		if err != nil {
			rel = p
		}
		relPaths = append(relPaths, "./"+rel)
	}

	outPath := cmdutil.FileSimulate
	if !dryRun && !force {
		if cmdutil.FileExists(outPath) {
			return fmt.Errorf("%s already exists — use --force to overwrite", outPath)
		}
	}

	type suiteMeta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description,omitempty"`
	}
	type suiteDoc struct {
		APIVersion string    `yaml:"apiVersion"`
		Kind       string    `yaml:"kind"`
		Metadata   suiteMeta `yaml:"metadata"`
		Imports    []string  `yaml:"imports"`
	}

	doc := suiteDoc{
		APIVersion: "inrun.dev/v1",
		Kind:       "Simulate",
		Metadata: suiteMeta{
			Name:        "suite",
			Description: fmt.Sprintf("Generated by inrun simulate init --suite — %d file(s) discovered", len(relPaths)),
		},
		Imports: relPaths,
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding suite: %w", err)
	}

	output := append([]byte("# Schema reference: "+cmdutil.SchemaRefSimulateSuite+"\n"), buf.Bytes()...)

	if dryRun {
		fmt.Print(string(output))
		return nil
	}

	if err := os.WriteFile(outPath, output, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	absRoot, _ := filepath.Abs(root)
	fmt.Printf("%s Generated %s\n", cmdutil.SuccessMark(), outPath)
	fmt.Printf("  %d file(s) discovered under %s\n", len(relPaths), absRoot)
	const maxShow = 10
	for i, p := range relPaths {
		if i >= maxShow {
			fmt.Printf("  ... %d more\n", len(relPaths)-maxShow)
			break
		}
		fmt.Printf("    %s\n", cmdutil.Dim(p))
	}
	fmt.Printf("\n  Run %s to verify.\n", cmdutil.Bold("inrun simulate"))
	return nil
}

// generateSimulateDoc builds a Simulate document from observed cycle-1 creates.
func generateSimulateDoc(catalogPath, crPath, catalogName string, results []crdOps) map[string]interface{} {
	trueVal := true

	makeOps := func(ops []simulate.Op) []map[string]interface{} {
		var rules []map[string]interface{}
		for _, op := range ops {
			rule := map[string]interface{}{
				"cycle":    1,
				"verb":     op.Verb,
				"resource": op.Resource,
			}
			if op.Name != "" {
				rule["name"] = op.Name
			}
			rules = append(rules, rule)
		}
		return rules
	}

	spec := map[string]interface{}{
		"catalog": catalogPath,
		"cr":      crPath,
		"cycles":  5,
	}

	if len(results) == 1 {
		expect := map[string]interface{}{
			"steady":   trueVal,
			"noErrors": trueVal,
		}
		if ops := makeOps(results[0].ops); len(ops) > 0 {
			expect["ops"] = ops
		}
		spec["expect"] = expect
	} else {
		crds := map[string]interface{}{}
		for _, r := range results {
			crdExpect := map[string]interface{}{
				"steady":   trueVal,
				"noErrors": trueVal,
			}
			if ops := makeOps(r.ops); len(ops) > 0 {
				crdExpect["ops"] = ops
			}
			crds[r.name] = crdExpect
		}
		spec["expect"] = map[string]interface{}{
			"noErrors": trueVal,
			"crds":     crds,
		}
	}

	return map[string]interface{}{
		"apiVersion": "inrun.dev/v1",
		"kind":       "Simulate",
		"metadata": map[string]interface{}{
			"name":        catalogName + "-sim",
			"description": "Generated by inrun simulate init — edit to refine",
		},
		"spec": spec,
	}
}

func countRules(results []crdOps) int {
	n := 0
	for _, r := range results {
		n += len(r.ops)
	}
	return n
}

// injectAbsentComment parses the encoded YAML, adds a HeadComment on every
// "steady" key hinting at the absent: block, then re-encodes with 2-space indent.
func injectAbsentComment(data []byte, results []crdOps) []byte {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return data
	}

	resource := "deployments"
	if len(results) > 0 && len(results[0].ops) > 0 {
		resource = results[0].ops[0].Resource
	}

	comment := "# absent:   # ops that must NOT appear — fill in for failure-path coverage\n" +
		"#   - cycle: 1\n" +
		"#     verb: create\n" +
		"#     resource: " + resource

	addHeadComment(&root, "steady", comment)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return data
	}
	return buf.Bytes()
}

// addHeadComment recursively sets HeadComment on every mapping key matching name.
func addHeadComment(node *yaml.Node, name, comment string) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content)-1; i += 2 {
			if node.Content[i].Value == name {
				node.Content[i].HeadComment = comment
			}
		}
	}
	for _, child := range node.Content {
		addHeadComment(child, name, comment)
	}
}

func init() {
	cmdutil.RootCmd.AddCommand(simulateCmd)
	simulateCmd.AddCommand(simulateInitCmd)

	simulateInitCmd.Flags().StringP("file", "f", "", "Path to catalog.yaml or stack.yaml")
	simulateInitCmd.Flags().String("cr", "", "Path to the CR YAML file")
	simulateInitCmd.Flags().Bool("force", false, "Overwrite existing simulate.yaml")
	simulateInitCmd.Flags().Bool("dry-run", false, "Print the generated simulate.yaml to stdout instead of writing the file")
	simulateInitCmd.Flags().Bool("suite", false, "Aggregate all simulate.yaml leaf files found under the given dir (default: .)")
	simulateInitCmd.Flags().StringSlice("skip", []string{}, "Comma-separated path patterns to exclude from suite discovery")

	simulateCmd.Flags().StringP("file", "f", "", "Path to catalog.yaml")
	simulateCmd.Flags().String("cr", "", "Path to the CR YAML file to simulate")
	simulateCmd.Flags().String("crd", "", "CRD name to simulate (default: all CRDs in Catalog)")
	simulateCmd.Flags().Int("cycles", 10, "Maximum number of reconcile cycles")
	simulateCmd.Flags().StringP("target", "t", "", "Target that provides the operatorbox for simulate")
	simulateCmd.Flags().StringSlice("skip", []string{}, "Comma-separated path patterns to skip during ./... discovery (e.g. vendor,cr-e2e.yaml)")
	simulateCmd.Flags().Bool("skip-external", false, "Stub external: HTTP calls with empty 200 responses instead of hitting the real network")
	simulateCmd.Flags().Bool("debug-ops", false, "Print every recorded op with its cycle number (diagnostic)")
	simulateCmd.Flags().Bool("verbose", false, "Print the reconciler's logs, and each stage of building a CR from spec.intent")
	simulateCmd.Flags().Bool("dev-server", false, "Start the mock dev server for external: examples")
	simulateCmd.Flags().Int("dev-server-port", devserver.Port, "Port for the mock dev server")
	simulateCmd.Flags().Bool("envtest", false, "Run against a local kube-apiserver + etcd instead of fake clients (binaries auto-downloaded to ~/.inrun/envtest-bins on first use)")
	simulateCmd.Flags().String("k8s-version", simulate.DefaultEnvtestK8sVersion, "Kubernetes version for envtest binaries (e.g. 1.31, 1.32); only used with --envtest")

	// Shadow global flags so they don't appear under `inrun simulate`
	cmdutil.ShadowGlobalCommandFlags(simulateCmd)
}

// projectFile returns name next to the Catalog, or in its sub directory dir
// when only that exists. The path next to the Catalog is the fallback.
func projectFile(catalogFile, name, dir string) string {
	base := filepath.Dir(catalogFile)
	if p := filepath.Join(base, dir, name); cmdutil.FileExists(p) && !cmdutil.FileExists(filepath.Join(base, name)) {
		return p
	}
	return filepath.Join(base, name)
}

// relFromDir returns path relative to dir (both resolved from the current
// directory), prefixed with ./ when it does not climb out of dir.
func relFromDir(dir, path string) string {
	absDir, err1 := filepath.Abs(dir)
	absPath, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return path
	}
	r, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return path
	}
	if !strings.HasPrefix(r, "..") {
		r = "./" + r
	}
	return r
}
