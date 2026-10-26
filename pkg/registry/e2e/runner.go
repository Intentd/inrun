package e2e

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/registry/e2e/expect/report"
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

	return r, nil
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

	var (
		appliedCRDPaths   []string
		appliedBundlePath string
		appliedSetupPaths []string
		installedInrun    bool
	)

	if needsCluster {
		if restore := preserveKubectlContext(); restore != nil {
			defer restore()
		}

		if !ownsCluster {
			defer func() {
				post.Teardown(context.Background(), post.TeardownConfig{
					BundlePath:     appliedBundlePath,
					CRDPaths:       appliedCRDPaths,
					SetupPaths:     appliedSetupPaths,
					Setup:          r.e2e.Spec.Setup,
					SharedInrun:    r.sharedInrun,
					UninstallInrun: installedInrun,
				})
			}()
		}

		if err := r.ensureCluster(ctx); err != nil {
			return nil, fmt.Errorf("cluster: %w", err)
		}
		if err := r.buildClient(); err != nil {
			return nil, fmt.Errorf("k8s client: %w", err)
		}
	}

	depsResult, err := r.ensureDependencies(ctx, isPureAgg, ownsCluster)
	if err != nil {
		return nil, err
	}

	appliedCRDPaths = depsResult.appliedCRDPaths
	appliedBundlePath = depsResult.appliedBundlePath
	appliedSetupPaths = depsResult.appliedSetupPaths
	installedInrun = depsResult.installedInrun

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
	r.printReport(ctx, name, result.Cases, result, isPureAgg)

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

	// Delete the parent-owned cluster after imports have completed.
	if r.needsCluster {
		if !r.useCurrentCtx && !r.keepCluster && r.clusterCtx == "" {
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
