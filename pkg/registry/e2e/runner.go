package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultClusterName = "inrun-e2e"
	defaultProvider    = "kind"
	defaultTimeout     = "60s"
)

// Runner executes a single E2E spec end-to-end.
type Runner struct {
	e2e           types.E2E
	e2eDir        string // directory of the e2e.yaml file — resolves relative paths
	keepCluster   bool
	useCurrentCtx bool   // Default (false) - means whether to use the current context, skip cluster creation
	clusterCtx    string // non-empty means use this context, skip cluster creation
	workers       int    // number of kind worker nodes to provision (0 = control-plane only)
	kindVersion   string // kind binary version to use ("" = DefaultKindVersion)

	blueprintFile string
	crFiles       []string

	// Inrun installation options
	inrunVersion string
	// inrunChart is a local chart directory to install instead of the published chart.
	inrunChart string
	valueFiles []string
	helmArgs   []string

	// devServer deploys the mock dev server into the cluster as part of setup.
	devServer bool

	// reportFile is the path to write the markdown results report to.
	// Empty means no file is written (stdout only).
	reportFile string

	// kubernetesTarget skips bundle generation and Inrun helm install/uninstall.
	// Set when spec.custom.target == "kubernetes" — the file is the source of truth.
	kubernetesTarget bool

	// sharedInrun means Inrun is managed by the parent runImports coordinator.
	// The sub-runner must not delete the bundle from the cluster (the bundle contains
	// the inrun-system namespace; deleting it cascades to the Inrun deployment).
	// It also suppresses all sync/health-check output — only the coordinator's single
	// install and uninstall messages are visible.
	sharedInrun bool

	// noRuntime skips starting the Inrun runtime. Only the gateway is started
	// when the blueprint enables it. The CR lifecycle steps (AfterCRApplied, AfterCRDeleted)
	// become no-ops — the CR is optional and gateway intents drive the expectations instead.
	noRuntime bool

	// cs and cfg are the Go Kubernetes client and REST config, built once after the
	// cluster context is ready. Used for operations that don't need kubectl:
	// Lease reads, port-forward+HTTP, SubjectAccessReview.
	cs  kubernetes.Interface
	cfg *rest.Config
}

// Options configures a Runner. All fields are optional — zero values produce
// the same behaviour as the previous positional defaults.
type Options struct {
	ClusterCtx    string   // use an existing kubectl context, skip cluster creation
	UseCurrentCtx bool     // use the current kubectl context as-is
	KeepCluster   bool     // do not delete the kind cluster after the run
	Workers       int      // number of kind worker nodes (0 = control-plane only)
	KindVersion   string   // kind binary version to download ("" = DefaultKindVersion)
	DevServer     bool     // deploy the mock dev server into the cluster
	InrunVersion  string   // Inrun helm chart version to install
	InrunChart    string   // local chart directory to install instead of the published chart
	ValueFiles    []string // additional Helm values files
	HelmArgs      []string // additional helm --set arguments
	ReportFile    string   // write results as markdown to this path (in addition to stdout)
	NoRuntime     bool     // skip the Inrun runtime; gateway only; CR is optional
}

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

	r := &Runner{
		e2e:              e2e,
		e2eDir:           e2eDir,
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
		noRuntime:        opts.NoRuntime,
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

// resolveSource resolves the blueprint and CR file paths from the spec.
func (r *Runner) resolveSource() error {
	spec := r.e2e.Spec

	switch {
	case spec.Init != nil:
		// Example pack — resolve from examples/ relative to e2e file or cwd.
		base := r.e2eDir
		candidate := filepath.Join(base, "examples", spec.Init.Pack, spec.Init.Example)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			// Try from cwd
			cwd, _ := os.Getwd()
			candidate = filepath.Join(cwd, "examples", spec.Init.Pack, spec.Init.Example)
		}
		r.blueprintFile = filepath.Join(candidate, "blueprint.yaml")
		r.crFiles = []string{filepath.Join(candidate, "cr.yaml")}

	case spec.Blueprint != "" && len(spec.AllCRPaths()) > 0:
		r.blueprintFile = r.abs(spec.Blueprint)
		for _, p := range spec.AllCRPaths() {
			r.crFiles = append(r.crFiles, r.abs(p))
		}

	case spec.Blueprint != "":
		// Blueprint without a CR: the CRs come from the gateway (an intent sent
		// by an expectation) or from the expectations themselves.
		r.blueprintFile = r.abs(spec.Blueprint)

	case spec.Custom != nil && spec.Custom.Target != "":
		// custom.target: both blueprint and cr are optional.
		for _, p := range spec.AllCRPaths() {
			r.crFiles = append(r.crFiles, r.abs(p))
		}

	case len(r.e2e.Imports) > 0:
		// Pure aggregator — no own blueprint/CR, just orchestrates imports.
		return nil

	default:
		return fmt.Errorf("e2e spec must declare a blueprint, init or custom target, or have imports")
	}

	if r.blueprintFile != "" {
		if _, err := os.Stat(r.blueprintFile); err != nil {
			return fmt.Errorf("blueprint file not found: %s", r.blueprintFile)
		}
	}
	for _, p := range r.crFiles {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("CR file not found: %s", p)
		}
	}
	return nil
}

// Run executes the full E2E test pipeline and returns a structured Result.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	name := r.e2e.Metadata.Name
	start := time.Now()
	if desc := r.e2e.Metadata.Description; desc != "" {
		fmt.Printf("\nRunning E2E: %s — %s\n\n", name, desc)
	} else {
		fmt.Printf("\nRunning E2E: %s\n\n", name)
	}

	// Capture the original kubectl context so we can restore it after the
	// test completes (the cluster creation or --cluster flag switches context).
	if origCtx, err := currentKubectlContext(); err == nil && origCtx != "" {
		defer func() {
			if out, err := exec.Command("kubectl", "config", "use-context", origCtx).CombinedOutput(); err != nil {
				fmt.Printf("  ! Could not restore kubectl context to %q: %v\n%s\n", origCtx, err, out)
			} else {
				fmt.Printf("\n→ kubectl context restored to %q\n", origCtx)
			}
		}()
	}

	// When e2e runs against an existing cluster (--cluster flag, --current-context or --keep-cluster),
	// it owns the resources it applied but not the cluster itself. Track everything
	// applied so it can be torn down in reverse order when the test completes.
	//
	// When e2e creates and deletes its own ephemeral cluster, teardown is handled
	// by the cluster deletion — no per-resource cleanup is needed.
	isPureAgg := r.isPureAggregator()

	ownsCluster := r.clusterCtx == "" && !r.keepCluster && !r.useCurrentCtx
	var (
		appliedCRDPaths   []string
		appliedBundlePath string
		appliedSetupPaths []string
		installedInrun    bool
	)
	if !ownsCluster {
		defer func() {
			r.teardown(context.Background(), appliedCRDPaths, appliedBundlePath, appliedSetupPaths, installedInrun)
		}()
	}

	// ── 1. Cluster ───────────────────────────────────────────────────────
	if err := r.ensureCluster(ctx); err != nil {
		return nil, fmt.Errorf("cluster: %w", err)
	}
	if err := r.buildClient(); err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	// Steps 2–9 are skipped for pure aggregators (no spec — imports only).
	// Each imported E2E runs its own full lifecycle against the shared cluster.
	var cases []CaseResult

	if !isPureAgg {
		// ── 2. Dependencies ──────────────────────────────────────────────
		fmt.Println("→ Ensuring dependencies...")
		if err := cluster.EnsureDependencies(); err != nil {
			return nil, fmt.Errorf("dependencies: %w", err)
		}

		// ── 3. Apply operator CRD ────────────────────────────────────────
		crdPaths, err := r.applyCRD(ctx)
		if err != nil {
			return nil, fmt.Errorf("applying CRD: %w", err)
		}
		appliedCRDPaths = crdPaths

		// ── 4. Pre-pull OCI imports ──────────────────────────────────────
		if !r.kubernetesTarget {
			if err := r.pullOCIImports(ctx); err != nil {
				return nil, fmt.Errorf("pulling OCI imports: %w", err)
			}
		}

		// ── 5. Generate and apply bundle ─────────────────────────────────
		if !r.kubernetesTarget {
			bundleFile, err := r.generateBundle(ctx)
			if err != nil {
				return nil, fmt.Errorf("generate bundle: %w", err)
			}
			if ownsCluster {
				defer os.Remove(bundleFile)
			} else {
				appliedBundlePath = bundleFile
			}

			fmt.Printf("→ Applying bundle...\n")
			if out, err := kubectl(ctx, "apply", "-f", bundleFile); err != nil {
				return nil, fmt.Errorf("apply bundle: %w\n%s", err, out)
			}
			fmt.Printf("  %s Bundle applied\n", successMark())
		}

		// ── 6. Setup ─────────────────────────────────────────────────────
		setupPaths, err := r.applySetup(ctx)
		if err != nil {
			return nil, fmt.Errorf("setup: %w", err)
		}
		appliedSetupPaths = setupPaths

		// ── 6b. Dev server ───────────────────────────────────────────────
		if r.devServer {
			devManifest, err := applyDevServer(ctx)
			if err != nil {
				return nil, fmt.Errorf("dev server: %w", err)
			}
			appliedSetupPaths = append(appliedSetupPaths, devManifest)
			fmt.Printf("→ Waiting for dev server to be ready...\n")
			if err := checkDevServerHealth(); err != nil {
				return nil, err
			}
			fmt.Printf("  %s Dev server ready\n", successMark())
		}

		// ── 7. Install Inrun ──────────────────────────────────────────
		// ── 8. Wait for Inrun ready ───────────────────────────────────
		// Both steps skipped when custom.target is set.
		if !r.kubernetesTarget {
			text := "..."

			// Console is never needed in e2e — disable it unconditionally.
			r.helmArgs = append(r.helmArgs, "--set", "console.enabled=false")

			gatewayEnabled, err := resolveGatewayEnabled(r.blueprintFile)
			if err != nil {
				return nil, err
			}
			if gatewayEnabled {
				fmt.Printf("→ Gateway enabled...\n")
				r.helmArgs = append(r.helmArgs, "--set", "gateway.enabled=true")
				text = " with gateway..."
			}
			if r.noRuntime {
				r.helmArgs = append(r.helmArgs, "--set", "runtime.enabled=false")
				text = " gateway only..."
			}

			if !cluster.RuntimeInstalled() {
				sp := startSpinner("Installing Inrun" + text)
				if err := cluster.InstallOrUpgradeInrun(r.inrunChart, r.inrunVersion, r.valueFiles, r.helmArgs...); err != nil {
					sp.Failure()
					return nil, fmt.Errorf("helm install: %w", err)
				}
				sp.Success()
				installedInrun = true
			} else {
				// Inrun is already running from a previous import.
				// Two steps are always needed:
				// 1. helm upgrade — applies this import's valueFiles (image, features).
				//    When values match the previous import the pod is not restarted.
				// 2. SyncRuntime — restarts the pod so it loads the new bundle ConfigMap.
				//    Without this, a values-identical upgrade leaves the old bundle in memory.
				sp := startSpinner("Upgrading Inrun" + text)
				if err := cluster.InstallOrUpgradeInrun(r.inrunChart, r.inrunVersion, r.valueFiles, r.helmArgs...); err != nil {
					sp.Failure()
					return nil, fmt.Errorf("helm upgrade: %w", err)
				}
				sp.Success()
				if !r.noRuntime {
					if err := cluster.SyncRuntime(); err != nil {
						return nil, fmt.Errorf("syncing Inrun runtime: %w", err)
					}
				}
				if gatewayEnabled {
					if cluster.GatewayInstalled() {
						if err := cluster.SyncGateway(); err != nil {
							return nil, fmt.Errorf("syncing Inrun gateway: %w", err)
						}
					} else {
						sp := startSpinner("Upgrading Inrun to enable gateway...")
						if err := cluster.InstallOrUpgradeInrun(r.inrunChart, r.inrunVersion, r.valueFiles, r.helmArgs...); err != nil {
							sp.Failure()
							return nil, fmt.Errorf("helm upgrade (gateway): %w", err)
						}
						sp.Success()
					}
				}
				installedInrun = true
			}

			if installedInrun {
				if !r.noRuntime {
					status := cluster.CheckRuntimeHealth()
					if !status.Running {
						return nil, fmt.Errorf("Inrun runtime not ready: %s", status.Reason)
					}
				}
				if gatewayEnabled {
					status := cluster.CheckGatewayHealth()
					if !status.Running {
						return nil, fmt.Errorf("Inrun gateway not ready: %s", status.Reason)
					}
				}
			}
		}

		// ── 9. Run expectations ──────────────────────────────────────────
		if err := ensureTools(r.e2e); err != nil {
			return nil, err
		}

		expects, err := ExpandExpectIncludes(r.e2e.Spec.Expect, r.e2eDir)
		if err != nil {
			return nil, err
		}

		// Build a template evaluator from spec.functions so when:/or: expressions
		// on expect blocks can reference user-defined functions.
		funcEval := template.NewResolverFromMap(nil).
			WithUserFunctions(r.e2e.Spec.Functions).
			TemplateEvaluator()

		crApplied := false
		crDeleted := false

		for _, exp := range expects {
			after := exp.After
			if after == "" {
				after = types.AfterSetupComplete
			}
			switch after {
			case types.AfterSetupComplete:
				// Infrastructure assertions — no CR lifecycle action needed.

			case types.AfterCRApplied:
				if !crApplied && !r.noRuntime {
					fmt.Printf("→ Applying CR(s)...\n")
					for _, p := range r.crFiles {
						if out, err := kubectl(ctx, "apply", "-f", p); err != nil {
							return nil, fmt.Errorf("apply CR %s: %w\n%s", p, err, out)
						}
					}
					fmt.Printf("  %s CR(s) applied\n\n", successMark())
					crApplied = true
				}

			case types.AfterCRDeleted:
				if !crDeleted && !r.noRuntime {
					fmt.Printf("→ Deleting CR(s)...\n")
					for _, p := range r.crFiles {
						if out, err := kubectl(ctx, "delete", "-f", p, "--ignore-not-found"); err != nil {
							return nil, fmt.Errorf("delete CR %s: %w\n%s", p, err, out)
						}
					}
					fmt.Printf("  %s CR(s) deleted\n\n", successMark())
					crDeleted = true
				}

			default:
				return nil, fmt.Errorf("unknown after: %q — valid values: %v", after, types.ValidAfterValues)
			}

			to := exp.Timeout
			if to == "" {
				to = defaultTimeout
			}
			fmt.Printf("  Waiting for %q (timeout: %s)...\n", exp.Name, to)
			caseStart := time.Now()
			waitDone := make(chan struct{})
			go func() {
				t := time.NewTicker(10 * time.Second)
				defer t.Stop()
				for {
					select {
					case <-waitDone:
						return
					case <-t.C:
						fmt.Printf("    still waiting [%s]...\n", time.Since(caseStart).Round(time.Second))
					}
				}
			}()
			workDir := r.e2eDir
			if exp.WorkDir != "" {
				workDir = exp.WorkDir
			}
			verifyErr := verifyExpectation(ctx, exp, workDir, r.cs, r.cfg, funcEval)
			close(waitDone)
			caseElapsed := time.Since(caseStart)

			skipped := verifyErr == errSkipped
			cr := CaseResult{
				Name:    exp.Name,
				Passed:  verifyErr == nil,
				Skipped: skipped,
				Elapsed: caseElapsed,
				Err:     verifyErr,
			}
			if skipped {
				cr.Err = nil
			}
			cases = append(cases, cr)
			switch {
			case skipped:
				fmt.Printf("  ~ %s (skipped)\n", exp.Name)
			case verifyErr != nil:
				fmt.Printf("  %s %s (%s): %v\n", failureMark(), exp.Name, caseElapsed.Round(time.Millisecond), verifyErr)
				runOnFailure(ctx, exp.OnFailure, r.e2eDir, r.cs)
			default:
				fmt.Printf("  %s %s (%s)\n", successMark(), exp.Name, caseElapsed.Round(time.Millisecond))
			}
		}
	}

	result := &Result{
		Name:    name,
		Cases:   cases,
		Elapsed: time.Since(start),
	}

	// ── Report ───────────────────────────────────────────────────────────
	if !isPureAgg {
		fmt.Printf("\nE2E Results: %s\n\n", name)

		for _, c := range cases {
			if c.Passed {
				fmt.Printf("  %s %-40s (%s)\n", successMark(), c.Name, c.Elapsed.Round(time.Millisecond))
			}
		}

		var failures []CaseResult
		for _, c := range cases {
			if !c.Passed && !c.Skipped {
				failures = append(failures, c)
			}
		}
		if len(failures) > 0 {
			fmt.Printf("\n")
			for _, c := range failures {
				fmt.Printf("  %s %-40s (%s)\n", failureMark(), c.Name, c.Elapsed.Round(time.Millisecond))
				if c.Err != nil {
					for _, line := range strings.Split(strings.TrimSpace(c.Err.Error()), "\n") {
						fmt.Printf("      %s\n", line)
					}
				}
			}
		}

		var skippedCases []CaseResult
		for _, c := range cases {
			if c.Skipped {
				skippedCases = append(skippedCases, c)
			}
		}
		if len(skippedCases) > 0 {
			fmt.Printf("\n")
			for _, c := range skippedCases {
				fmt.Printf("  ~ %-40s (skipped)\n", c.Name)
			}
		}

		clusterInfo := r.clusterName()
		fmt.Printf("\n  %s\n", result.Summary())
		if clusterInfo != "" {
			fmt.Printf("  Cluster: %s (%s)\n", clusterInfo, r.provider())
		}

		if len(failures) > 0 {
			runOnFailure(ctx, r.e2e.Spec.OnFailure, r.e2eDir, r.cs)
		}

		if r.reportFile != "" {
			if err := os.WriteFile(r.reportFile, []byte(result.Markdown()), 0644); err != nil {
				fmt.Printf("  ! could not write report file: %v\n", err)
			} else {
				fmt.Printf("  Report written to %s\n", r.reportFile)
			}
		}
	}

	// ── 10. Imports ──────────────────────────────────────────────────────
	var importErr error
	importCount := len(r.e2e.Imports)
	importText := "imports"

	if importCount == 1 {
		importText = "import"
	}

	if importCount > 0 {
		fmt.Printf("\n─── Running %d %s ───\n", importCount, importText)
		importResults := r.runImports(ctx)
		importErr = printImportSummary(r.e2e.Metadata.Name, importResults)
	}

	// ── 11. Cleanup ──────────────────────────────────────────────────────
	if !r.useCurrentCtx && !r.keepCluster && r.clusterCtx == "" {
		fmt.Printf("\n→ Deleting cluster '%s'...\n", r.clusterName())
		if err := r.deleteCluster(ctx); err != nil {
			fmt.Printf("  ! Could not delete cluster: %v\n", err)
		} else {
			fmt.Printf("  %s Cluster deleted\n", successMark())
		}
	}

	if !result.AllPassed() {
		return result, fmt.Errorf("%d of %d expectations failed", result.Total()-result.Passed(), result.Total())
	}
	return result, importErr
}

// resolveGatewayEnabled inspects the blueprint file and returns true if the
// gateway block is present. This allows Helm installation to automatically
// enable the gateway chart when required by the blueprint.
func resolveGatewayEnabled(blueprintFile string) (bool, error) {
	var raw struct {
		Gateway *types.GatewayConfig `yaml:"gateway,omitempty"`
	}

	data, err := readLocal(blueprintFile)
	if err != nil {
		return false, err
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	return raw.Gateway != nil, nil
}

// buildClient constructs the Go Kubernetes client and REST config from the active
// kubeconfig after the cluster context has been established by ensureCluster.
func (r *Runner) buildClient() error {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	r.cfg = cfg
	r.cs = cs
	return nil
}

// ensureCluster sets up the cluster according to the spec.
func (r *Runner) ensureCluster(ctx context.Context) error {
	if r.useCurrentCtx {
		fmt.Printf("→ Using current cluster context...\n")
		return nil
	}

	if r.clusterCtx != "" {
		fmt.Printf("→ Using existing cluster context %q...\n", r.clusterCtx)
		if out, err := exec.CommandContext(ctx, "kubectl", "config", "use-context", r.clusterCtx).CombinedOutput(); err != nil {
			return fmt.Errorf("switching context: %w\n%s", err, out)
		}
		fmt.Printf("  %s Using context %s\n", successMark(), r.clusterCtx)
		return nil
	}

	provider := r.provider()
	if provider != "kind" {
		return fmt.Errorf("provider %q not supported — only 'kind' is available", provider)
	}

	name := r.clusterName()
	spec := r.e2e.Spec.Cluster

	if !spec.Reuse {
		// Delete if already exists for a clean state
		if clusterExists(name) {
			fmt.Printf("→ Recreating cluster '%s' (reuse: false)...\n", name)
			if err := deleteKindCluster(ctx, name); err != nil {
				return fmt.Errorf("deleting old cluster: %w", err)
			}
		}
	}

	return cluster.EnsureKindCluster(name, r.workers, r.kindVersion)
}

// applyCRD applies the operator's CRD to the cluster and returns the paths applied.
// Uses spec.AllCRDPaths() if declared; falls back to crdFile entries in the blueprint.
func (r *Runner) applyCRD(ctx context.Context) ([]string, error) {
	if crdPaths := r.e2e.Spec.AllCRDPaths(); len(crdPaths) > 0 {
		var applied []string
		for _, crd := range crdPaths {
			path := r.abs(crd)
			fmt.Printf("→ Applying CRD from %s...\n", crd)
			if out, err := kubectl(ctx, "apply", "-f", path); err != nil {
				return nil, fmt.Errorf("applying CRD %s: %w\n%s", crd, err, out)
			}
			applied = append(applied, path)
		}
		fmt.Printf("  %s CRD(s) applied\n", successMark())
		return applied, nil
	}

	// Fallback: read crdFile references from the blueprint.
	// When kubernetesTarget is true and no blueprint is provided, there is nothing to fall back to.
	if r.blueprintFile == "" {
		return nil, nil
	}
	var raw struct {
		Spec struct {
			CRDs map[string]struct {
				CRDFile string `yaml:"crdFile"`
			} `yaml:"crds"`
		} `yaml:"spec"`
	}
	data, err := readLocal(r.blueprintFile)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	blueprintDir := filepath.Dir(r.blueprintFile)
	var applied []string
	for name, entry := range raw.Spec.CRDs {
		if entry.CRDFile == "" {
			continue
		}
		path := entry.CRDFile
		if !filepath.IsAbs(path) && !strings.HasPrefix(path, "http") {
			path = filepath.Join(blueprintDir, path)
		}
		fmt.Printf("→ Applying CRD '%s' from %s...\n", name, entry.CRDFile)
		if out, err := kubectl(ctx, "apply", "-f", path); err != nil {
			fmt.Printf("  ! CRD apply failed (continuing): %v\n%s\n", err, out)
		} else {
			fmt.Printf("  %s CRD applied\n", successMark())
			applied = append(applied, path)
		}
	}
	return applied, nil
}

func (r *Runner) generateBundle(ctx context.Context) (string, error) {
	// Anchor the blueprint directory as an absolute path. r.blueprintFile may be
	// relative when inrun e2e is invoked without an explicit -f path; all temp
	// file creation and cmd.Dir must use an absolute base to avoid double-nested
	// paths when cmd.Dir is set.
	blueprintDir, err := filepath.Abs(filepath.Dir(r.blueprintFile))
	if err != nil {
		return "", fmt.Errorf("resolving blueprint directory: %w", err)
	}

	// Resolve any crdFile references to inline apiTypes before bundling.
	// The Inrun runtime runs inside a container and cannot read local files —
	// all type information must be embedded in the ConfigMap.
	resolved, err := blueprint.ResolveCRDFiles(r.blueprintFile)
	if err != nil {
		return "", fmt.Errorf("resolving crdFile references: %w", err)
	}

	// Create the temp file in the blueprint's directory (absolute) so that
	// relative imports.files paths resolve correctly when inrun generate bundle runs.
	resolvedBlueprint, err := os.CreateTemp(blueprintDir, "inrun-e2e-blueprint-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := resolvedBlueprint.Write(resolved); err != nil {
		resolvedBlueprint.Close()
		os.Remove(resolvedBlueprint.Name())
		return "", err
	}
	resolvedBlueprint.Close()
	defer os.Remove(resolvedBlueprint.Name())

	bundleFile, err := os.CreateTemp("", "inrun-e2e-bundle-*.yaml")
	if err != nil {
		return "", err
	}
	bundleFile.Close()

	fmt.Printf("→ Generating bundle from %s...\n", r.blueprintFile)
	inrunBin, err := os.Executable()
	if err != nil {
		inrunBin = "inrun"
	}
	cmd := exec.CommandContext(ctx, inrunBin, "generate", "bundle",
		"-f", resolvedBlueprint.Name(),
		"-o", bundleFile.Name(),
	)
	// Run from the blueprint's directory (absolute) so relative imports.files
	// paths (e.g. ./platform-team/blueprint.yaml) resolve correctly.
	cmd.Dir = blueprintDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(bundleFile.Name())
		return "", fmt.Errorf("inrun generate bundle: %w", err)
	}
	fmt.Printf("  %s Bundle generated\n", successMark())
	return bundleFile.Name(), nil
}

// pullOCIImports pre-pulls all OCI module and registry imports referenced in
// the blueprint file so that bundle generation never needs to do OCI calls.
// Uses the same resolution logic as the merge path, including bare-name shorthands.
func (r *Runner) pullOCIImports(_ context.Context) error {
	imports, err := registry.ExtractOCIImports(r.blueprintFile)
	if err != nil {
		return fmt.Errorf("extracting OCI imports from %s: %w", r.blueprintFile, err)
	}
	if imports.Empty() {
		return nil
	}

	fmt.Printf("→ Pulling OCI imports...\n")

	for _, imp := range imports.ModuleImports {
		fmt.Printf("  → module %s\n", imp.Module)
		if err := module.PullImport(&imp); err != nil {
			return fmt.Errorf("pulling module %q: %w", imp.Module, err)
		}
		fmt.Printf("  %s %s\n", successMark(), imp.Module)
	}

	client, err := registry.NewClient()
	if err != nil {
		return fmt.Errorf("initializing registry client: %w", err)
	}
	_ = client // used for registry source pulls below when Stack support is needed

	return nil
}

func (r *Runner) applySetup(ctx context.Context) ([]string, error) {
	s := r.e2e.Spec.Setup
	if s == nil {
		return nil, nil
	}

	var applied []string

	// ── Phase 1: apply ────────────────────────────────────────────────────────
	for _, entry := range s.Apply {
		abs := r.abs(entry.Path)
		fmt.Printf("→ Applying setup %s...\n", entry.Path)
		if out, err := kubectl(ctx, "apply", "-f", abs); err != nil {
			return applied, fmt.Errorf("setup apply %s: %w\n%s", entry.Path, err, out)
		}
		fmt.Printf("  %s Applied\n", successMark())
		applied = append(applied, abs)
		for _, w := range entry.Wait {
			if err := runSetupWait(ctx, w); err != nil {
				return applied, fmt.Errorf("setup apply %s wait: %w", entry.Path, err)
			}
		}
	}

	// ── Phase 2: helm ─────────────────────────────────────────────────────────
	for _, h := range s.Helm {
		if h.IsLocalChart() && !filepath.IsAbs(h.Chart) {
			h.Chart = r.abs(h.Chart)
		}
		for i, f := range h.ValueFiles {
			if f != "" && !filepath.IsAbs(f) {
				h.ValueFiles[i] = r.abs(f)
			}
		}
		sp := startSpinner(fmt.Sprintf("Installing %s...", h.ReleaseName()))
		if err := cluster.HelmInstall(ctx, h); err != nil {
			sp.Failure()
			return applied, fmt.Errorf("setup helm %s: %w", h.Chart, err)
		}
		sp.Success()
		for _, w := range h.Wait {
			if err := runSetupWait(ctx, w); err != nil {
				return applied, fmt.Errorf("setup helm %s wait: %w", h.ReleaseName(), err)
			}
		}
	}

	// ── Phase 3: wait ─────────────────────────────────────────────────────────
	for _, w := range s.Wait {
		if err := runSetupWait(ctx, w); err != nil {
			return applied, fmt.Errorf("setup wait: %w", err)
		}
	}

	return applied, nil
}

func runSetupWait(ctx context.Context, w types.SetupWait) error {
	loc := w.Kind + " " + w.Name
	if w.Namespace != "" {
		loc += " (" + w.Namespace + ")"
	}
	sp := startSpinner(fmt.Sprintf("Waiting for %s...", loc))
	if err := cluster.WaitForResource(ctx, w); err != nil {
		sp.Failure()
		return err
	}
	sp.Success()
	return nil
}

func (r *Runner) deleteCluster(ctx context.Context) error {
	return deleteKindCluster(ctx, r.clusterName())
}

func (r *Runner) clusterName() string {
	if r.e2e.Spec.Cluster.Name != "" {
		return r.e2e.Spec.Cluster.Name
	}
	return defaultClusterName
}

func (r *Runner) provider() string {
	if r.e2e.Spec.Cluster.Provider != "" {
		return r.e2e.Spec.Cluster.Provider
	}
	return defaultProvider
}

// isPureAggregator returns true when this E2E has no spec of its own —
// it exists only to run imported E2E files.
// A kubernetesTarget spec is never a pure aggregator even when cr and blueprint are omitted.
func (r *Runner) isPureAggregator() bool {
	return r.blueprintFile == "" && len(r.crFiles) == 0 && !r.kubernetesTarget
}

func (r *Runner) abs(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(r.e2eDir, path)
}

func currentKubectlContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimSpace(string(out)), err
}

// kubectl runs a kubectl command and returns combined output.
func kubectl(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "kubectl", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func clusterExists(name string) bool {
	out, _ := exec.Command("kind", "get", "clusters").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// validateImports is a backstop that delegates to the exported ValidateImports.
// inrun e2e always calls this at startup so malformed imports are caught before
// the cluster is provisioned. inrun validate calls ValidateImports directly for
// earlier, friendlier feedback.
func (r *Runner) validateImports() error {
	errs := ValidateImports(r.e2eDir, r.e2e.Imports)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("invalid imports: %s", strings.Join(msgs, "; "))
}

// runImports runs each imported E2E file after the main test completes.
//
// Cluster strategy for shared-cluster imports (freshCluster: false, the default):
//   - Parent is a pure aggregator: imports use the cluster ensureCluster already set up.
//   - Parent used --use-current or --cluster: imports reuse the same active context.
//   - Parent ran its own test and created a kind cluster: a separate kind cluster
//     is created so imports don't share state with the parent's live resources.
//
// Imports with freshCluster: true always provision their own independent cluster.
func (r *Runner) runImports(ctx context.Context) []ImportResult {
	parentOwnsCluster := r.clusterCtx == "" && !r.useCurrentCtx

	if parentOwnsCluster && !r.isPureAggregator() {
		// Non-aggregator parent created its own cluster — provision a separate
		// one so imports don't run alongside the parent's Inrun install.
		importCluster := r.clusterName() + "-imports"
		fmt.Printf("→ Creating imports cluster '%s'...\n", importCluster)
		if err := cluster.EnsureKindCluster(importCluster, r.workers, r.kindVersion); err != nil {
			return []ImportResult{{Path: importCluster, Err: fmt.Errorf("creating imports cluster: %w", err)}}
		}
		if !r.keepCluster {
			defer func() {
				fmt.Printf("→ Deleting imports cluster '%s'...\n", importCluster)
				_ = deleteKindCluster(ctx, importCluster)
			}()
		}
	}

	// Each sub-runner installs or upgrades Inrun with its own valueFiles so
	// fixture-specific values (image, features) are applied correctly. The
	// coordinator only owns cleanup — it defers an uninstall that runs after all
	// imports complete. --ignore-not-found makes it safe if no import ran.
	if !r.kubernetesTarget {
		hasShared := false
		for _, imp := range r.e2e.Imports {
			if !imp.FreshCluster {
				hasShared = true
				break
			}
		}
		if hasShared {
			defer func() {
				fmt.Printf("→ Uninstalling Inrun...\n")
				cmd := exec.CommandContext(ctx, "helm", "uninstall", cluster.Inrun,
					"--namespace", cluster.InrunNamespace, "--ignore-not-found")
				if out, err := cmd.CombinedOutput(); err != nil {
					fmt.Printf("  ! helm uninstall failed: %v\n%s\n", err, out)
				} else {
					fmt.Printf("  %s Inrun uninstalled\n", successMark())
				}
			}()
		}
	}

	var results []ImportResult
	for _, imp := range r.e2e.Imports {
		absPath := r.abs(imp.Path)
		ir := ImportResult{Path: imp.Path}

		if imp.Wait != "" {
			d, _ := parseTimeDuration(imp.Wait) // already validated at load time
			fmt.Printf("→ Waiting %s before %s...\n", imp.Wait, filepath.Base(imp.Path))
			time.Sleep(d)
		}

		var sub *Runner
		var err error
		if imp.FreshCluster {
			sub, err = New(absPath, Options{KeepCluster: r.keepCluster, Workers: r.workers, KindVersion: r.kindVersion, DevServer: r.devServer, InrunVersion: r.inrunVersion, InrunChart: r.inrunChart, ValueFiles: r.valueFiles})
		} else {
			sub, err = New(absPath, Options{UseCurrentCtx: true, Workers: r.workers, KindVersion: r.kindVersion, DevServer: r.devServer, InrunVersion: r.inrunVersion, InrunChart: r.inrunChart, ValueFiles: r.valueFiles})
			if err == nil {
				sub.sharedInrun = true
			}
		}
		// kubernetesTarget is declared in the sub-file itself; the parent's value
		// does not override it — each import is authoritative about its own mode.
		if err != nil {
			ir.Err = fmt.Errorf("loading import %s: %w", imp.Path, err)
			results = append(results, ir)
			continue
		}
		res, runErr := sub.Run(ctx)
		ir.Result = res
		ir.Err = runErr
		results = append(results, ir)
	}
	return results
}

func deleteKindCluster(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
