package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/tools/cluster"
)

// runImports runs each imported E2E file after the main test completes.
//
// Cluster strategy for shared-cluster imports (freshCluster: false, the default):
//   - Parent is a pure aggregator: imports use the cluster ensureCluster already set up.
//   - Parent used --use-current or --cluster: imports reuse the same active context.
//   - Parent ran its own test and created a kind cluster: a separate kind cluster
//     is created so imports don't share state with the parent's live resources.
//
// Imports with freshCluster: true always provision their own independent cluster.
func (r *Runner) runImports(ctx context.Context) expect.ImportResults {
	if r.needsCluster {
		cleanup, failure := r.prepareImportsCluster(ctx)
		if failure != nil {
			return failure
		}
		if cleanup != nil {
			defer cleanup()
		}
	}

	var results []expect.ImportResult
	for _, imp := range r.e2e.Imports {
		absPath := r.abs(imp.Path)
		ir := expect.ImportResult{Path: imp.Path}

		if imp.Wait != "" {
			d, _ := parseTimeDuration(imp.Wait) // already validated at load time
			fmt.Printf("→ Waiting %s before %s...\n", imp.Wait, filepath.Base(imp.Path))
			time.Sleep(d)
		}

		var sub *Runner
		var err error
		opts := Options{
			Env:          r.e2e.Spec.Env,
			Workers:      r.workers,
			KindVersion:  r.kindVersion,
			DevServer:    r.devServer,
			InrunVersion: r.inrunVersion,
			InrunChart:   r.inrunChart,
			ValueFiles:   r.valueFiles,
			HelmArgs:     r.helmArgs,
			NoRuntime:    r.noRuntime,
		}

		if imp.FreshCluster {
			opts.KeepCluster = r.keepCluster
		} else {
			opts.UseCurrentCtx = true
		}

		sub, err = New(absPath, opts)
		if err == nil {
			sub.suppressReport = true

			if !imp.FreshCluster {
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

// prepareImportsCluster handles cluster setup for imports and returns a cleanup
// function the caller must defer. A non-nil failure slice means setup failed and
// the caller should return it as the import result directly.
//
// Cluster strategy for shared-cluster imports (freshCluster: false, the default):
//   - Parent is a pure aggregator: imports use the cluster ensureCluster already set up.
//   - Parent used --use-current or --cluster: imports reuse the same active context.
//   - Parent ran its own test and created a kind cluster: a separate kind cluster
//     is created so imports don't share state with the parent's live resources.
//
// Imports with freshCluster: true always provision their own independent cluster.
func (r *Runner) prepareImportsCluster(ctx context.Context) (cleanup func(), failure []expect.ImportResult) {
	if !r.needsCluster {
		return nil, nil
	}

	parentOwnsCluster := r.clusterCtx == "" && !r.useCurrentCtx
	if parentOwnsCluster && !r.isPureAggregator() {
		// Non-aggregator parent created its own cluster — provision a separate
		// one so imports don't run alongside the parent's Inrun install.
		importCluster := r.clusterName() + "-imports"
		fmt.Printf("→ Creating imports cluster '%s'...\n", importCluster)
		if err := cluster.EnsureKindCluster(importCluster, r.workers, r.kindVersion); err != nil {
			return nil, []expect.ImportResult{{
				Path: importCluster,
				Err:  fmt.Errorf("creating imports cluster: %w", err),
			}}
		}
		if !r.keepCluster {
			cleanup = func() {
				fmt.Printf("→ Deleting imports cluster '%s'...\n", importCluster)
				_ = deleteKindCluster(ctx, importCluster)
			}
		}
	}

	// Each sub-runner installs or upgrades Inrun with its own valueFiles so
	// fixture-specific values (image, features) are applied correctly. The
	// coordinator only owns cleanup — it defers an uninstall that runs after all
	// imports complete. --ignore-not-found makes it safe if no import ran.
	if !r.kubernetesTarget && r.hasSharedClusterImports() {
		prevCleanup := cleanup
		cleanup = func() {
			if prevCleanup != nil {
				prevCleanup()
			}
			fmt.Printf("→ Uninstalling Inrun...\n")
			cmd := exec.CommandContext(ctx, "helm", "uninstall", cluster.Inrun,
				"--namespace", cluster.InrunNamespace, "--ignore-not-found")
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("  ! helm uninstall failed: %v\n%s\n", err, out)
			} else {
				fmt.Printf("  %s Inrun uninstalled\n", successMark())
			}
		}
	}

	return cleanup, nil
}

// hasSharedClusterImports reports whether any import reuses the parent's
// cluster (freshCluster: false).
func (r *Runner) hasSharedClusterImports() bool {
	for _, imp := range r.e2e.Imports {
		if !imp.FreshCluster {
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
