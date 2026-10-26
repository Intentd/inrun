package e2e

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/registry/e2e/expect/report"
	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/registry/e2e/post"
	"github.com/intentd/inrun/pkg/types"
)

// New loads an E2E spec from a YAML file and constructs a Runner.
func New(e2eFile string, opts Options) (*Runner, error) {
	data, err := readLocal(e2eFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", e2eFile, err)
	}

	var e2e types.E2E
	if err := strictUnmarshal(data, &e2e); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", e2eFile, err)
	}
	if e2e.Kind != "E2E" {
		return nil, fmt.Errorf("%s: expected kind E2E, got %q", e2eFile, e2e.Kind)
	}

	e2eDir, err := filepath.Abs(filepath.Dir(e2eFile))
	if err != nil {
		return nil, fmt.Errorf("resolving e2e directory: %w", err)
	}
	allValueFiles := make([]string, 0, len(e2e.Spec.ValuesFiles)+len(opts.ValueFiles))
	for _, f := range e2e.Spec.ValuesFiles {
		if !filepath.IsAbs(f) {
			f = filepath.Join(e2eDir, f)
		}
		allValueFiles = append(allValueFiles, f)
	}
	allValueFiles = append(allValueFiles, opts.ValueFiles...)

	noRuntime := opts.NoRuntime || !e2e.Spec.RuntimeEnabled()

	// pass inheritance
	e2e.Spec.InheritEnv(opts.Env)

	r := &Runner{
		e2e:              e2e,
		e2eDir:           e2eDir,
		e2eFile:          e2eFile,
		keepCluster:      opts.KeepCluster,
		clusterCtx:       opts.ClusterCtx,
		useCurrentCtx:    opts.UseCurrentCtx,
		workers:          opts.Workers,
		kindVersion:      opts.KindVersion,
		devServer:        opts.DevServer,
		reportFile:       opts.ReportFile,
		inrunVersion:     opts.InrunVersion,
		inrunChart:       opts.InrunChart,
		valueFiles:       allValueFiles,
		helmArgs:         opts.HelmArgs,
		kubernetesTarget: e2e.Spec.Custom != nil && e2e.Spec.Custom.Target == types.CustomTargetKubernetes,
		noRuntime:        noRuntime,
		command:          opts.Command,
		dryRun:           opts.DryRun,
		maxFailures:      opts.MaxFailures,
		pushTriggered:    opts.PushTriggered,
	}

	if e2e.Spec.Custom != nil && e2e.Spec.Custom.Target == types.CustomTargetContainer {
		return nil, fmt.Errorf("spec.custom.target \"container\" is coming soon — not yet supported in this version")
	}

	if err := r.resolveSource(); err != nil {
		return nil, err
	}
	if err := r.validateImports(); err != nil {
		return nil, err
	}

	// Merge blueprint functions into the E2E spec so expectations can use them.
	// E2E-level definitions win on name collision — they are the test's overrides.
	if r.blueprintFile != "" {
		bp, err := blueprint.ParseFile(r.blueprintFile)
		if err != nil {
			return nil, fmt.Errorf("parsing blueprint for functions: %w", err)
		}
		merged, err := r.e2e.Spec.Functions.Merge(bp.UserFunctions(), "blueprint")
		if err != nil {
			return nil, fmt.Errorf("merging blueprint functions: %w", err)
		}
		r.e2e.Spec.Functions = merged
	}

	return r, nil
}

// suiteOnFailure runs the spec's onFailure once for the suite: after its
// expectations when any failed, or when setup failed before they ran. Not for
// a pure aggregator, whose imports run their own, nor under a server dry run,
// which changed nothing to diagnose. It returns the block's warnings.
func (r *Runner) suiteOnFailure(ctx context.Context, isPureAgg bool) []string {
	if isPureAgg || kubectl.DryRun() {
		return nil
	}
	spec := &r.e2e.Spec
	if spec.OnFailure == nil {
		return nil
	}
	return expect.SuitePost(ctx, "onFailure", &spec.OnFailure.E2ESideEffects, r.suitePostOptions())
}

// suiteFinally runs the spec's finally once, at the end of the suite, whatever
// happened before it. Not under a server dry run, which changed nothing to
// undo. It returns the block's warnings.
func (r *Runner) suiteFinally(ctx context.Context) []string {
	spec := &r.e2e.Spec
	if spec.Finally == nil || kubectl.DryRun() {
		return nil
	}
	return expect.SuitePost(ctx, "finally", &spec.Finally.E2ESideEffects, r.suitePostOptions())
}

func (r *Runner) suitePostOptions() expect.SuitePostOptions {
	return expect.SuitePostOptions{
		Spec:    &r.e2e.Spec,
		WorkDir: r.e2eDir,
		CS:      r.cs,
		Cfg:     r.cfg,
	}
}

// Run executes the full E2E test pipeline and returns a structured Result.
func (r *Runner) Run(ctx context.Context) (*expect.Result, error) {
	name := r.e2e.Metadata.Name
	start := time.Now()

	if desc := r.e2e.Metadata.Description; desc != "" {
		fmt.Printf("\nRunning E2E: %s — %s\n\n", name, desc)
	} else {
		fmt.Printf("\nRunning E2E: %s\n\n", name)
	}

	isPureAgg := r.isPureAggregator()
	ownsCluster := r.clusterCtx == "" && !r.keepCluster && !r.useCurrentCtx

	needsCluster, err := r.requiresCluster()
	if err != nil {
		return nil, err
	}
	r.needsCluster = needsCluster

	// --dry-run=server: every kubectl write, setup included, goes to an
	// existing cluster as a server dry run; nothing is created or installed.
	if r.dryRun {
		if !needsCluster {
			return nil, fmt.Errorf("--dry-run=server sends writes to a cluster, but this spec runs without one (cluster.provider: none)")
		}
		if ownsCluster {
			return nil, fmt.Errorf("--dry-run=server needs an existing cluster: use --use-current or --cluster <context>")
		}
		kubectl.SetDryRun(true)
		defer kubectl.SetDryRun(false)
		fmt.Println("→ Dry run (server): writes are checked by the API server, not applied; commands and assertions are skipped")
	}

	var (
		appliedCRDPaths   []string
		appliedBundlePath string
		appliedSetupPaths []string
		installedInrun    bool
		clusterDeleted    bool
	)

	if needsCluster {
		if restore := preserveKubectlContext(); restore != nil {
			defer restore()
		}

		if !ownsCluster {
			defer func() {
				// The CRs exist only once their CRDs do.
				var crPaths []string
				if len(appliedCRDPaths) > 0 {
					crPaths = r.crFiles
				}
				post.Teardown(context.Background(), post.TeardownConfig{
					CRPaths:        crPaths,
					BundlePath:     appliedBundlePath,
					CRDPaths:       appliedCRDPaths,
					SetupPaths:     appliedSetupPaths,
					Setup:          r.e2e.Spec.Setup,
					SharedInrun:    r.sharedInrun,
					UninstallInrun: installedInrun,
				})
			}()
		}

		// A cluster this run creates is deleted even when the run stops
		// early; the normal path deletes it below, after the imports.
		if ownsCluster {
			defer func() {
				if !clusterDeleted {
					fmt.Printf("\n→ Deleting cluster '%s' (run stopped early)...\n", r.clusterName())
					if err := r.deleteCluster(context.Background()); err != nil {
						fmt.Printf("  ! Could not delete cluster: %v\n", err)
					}
				}
			}()
		}
		if err := r.ensureCluster(ctx); err != nil {
			return nil, fmt.Errorf("cluster: %w", err)
		}
		if err := r.buildClient(); err != nil {
			return nil, fmt.Errorf("k8s client: %w", err)
		}
	}

	// The spec's finally runs once: on the normal path just before the report,
	// so its warnings are in it, and on any earlier return through this defer.
	// Deferred after the teardown and cluster defers, so it runs before them.
	finallyRan := false
	runFinally := func() []string {
		if finallyRan {
			return nil
		}
		finallyRan = true
		return r.suiteFinally(ctx)
	}
	defer runFinally()

	// What was applied is recorded before the error check, so teardown
	// removes it when a later dependency fails.
	depsResult, err := r.ensureDependencies(ctx, isPureAgg, ownsCluster)
	if depsResult != nil {
		appliedCRDPaths = depsResult.appliedCRDPaths
		appliedBundlePath = depsResult.appliedBundlePath
		appliedSetupPaths = depsResult.appliedSetupPaths
		installedInrun = depsResult.installedInrun
	}
	if err != nil {
		// A failed setup is the failure diagnostics help most with.
		r.suiteOnFailure(ctx, isPureAgg)
		return nil, err
	}

	expects := depsResult.expects
	planned := len(expects)

	exp, err := r.runExpectations(ctx, name, start, expects, planned)
	if err != nil {
		return nil, err
	}

	result := exp.result
	if result == nil {
		return nil, fmt.Errorf("expectations runner returned a nil result")
	}

	stopped := exp.stopped
	failures := exp.failures

	failureStr := "failure"
	if failures != 1 {
		failureStr = "failures"
	}

	if stopped != nil {
		fmt.Printf("\n  ! stopped after %d %s (%s) — %d of %d expectations run\n",
			failures, failureStr, result.StopReason, result.Total(), planned)
	}

	// Print the parent suite's console report before running imports.
	r.printReport(name, result.Cases, result, isPureAgg)
	if !result.AllPassed() {
		result.Warnings = r.suiteOnFailure(ctx, isPureAgg)
	}

	var runErr error
	if stopped != nil {
		runErr = fmt.Errorf("%d of %d expectations failed (%d of %d run): %w",
			result.Failed(), result.Total(), result.Total(), planned, stopped)
	} else if !result.AllPassed() {
		runErr = fmt.Errorf("%d of %d expectations failed",
			result.Failed(), result.Total())
	}

	// Run imports and attach their results to the parent report.
	count := len(r.e2e.Imports)
	if count > 0 {
		importText := "imports"
		if count == 1 {
			importText = "import"
		}

		fmt.Printf("\n─── Running %d %s ───\n", count, importText)

		importResults := r.runImports(ctx)
		result.Imports = importResults

		if importErr := importResults.PrintImportSummary(name); importErr != nil {
			runErr = errors.Join(runErr, importErr)
		}
	}

	result.Warnings = append(result.Warnings, runFinally()...)

	// Delete the parent-owned cluster after imports have completed.
	if r.needsCluster {
		if ownsCluster {
			clusterDeleted = true
			fmt.Printf("\n→ Deleting cluster '%s'...\n", r.clusterName())
			if err := r.deleteCluster(ctx); err != nil {
				fmt.Printf("  ! Could not delete cluster: %v\n", err)
				runErr = errors.Join(runErr, fmt.Errorf("deleting cluster: %w", err))
			} else {
				fmt.Printf("  %s Cluster deleted\n", successMark())
			}
		}
	}

	// Only the root runner writes a report. Imported runners contribute
	// their results to this report instead of creating their own files.
	if !r.suppressReport {
		if err := report.Write(r.reportFile, result); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("writing report: %w", err))
		}
	}

	return result, runErr
}
