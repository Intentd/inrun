package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/tools/cluster"
)

func (r *Runner) ensureDependencies(
	ctx context.Context,
	isPureAgg bool,
	ownsCluster bool,
) (*dependencyResult, error) {
	result := &dependencyResult{}

	if isPureAgg {
		return result, nil
	}

	// Everything a run needs regardless of cluster: expand includes and
	// check the tools the expectations will call.
	var err error
	result.expects, err = r.e2e.Spec.Expect.ExpandIncludes(r.e2eDir)
	if err != nil {
		return nil, err
	}
	if err := ensureTools(r.e2e); err != nil {
		return nil, err
	}

	// No cluster: commands-only. Nothing below applies.
	if !r.needsCluster {
		return result, nil
	}

	// A server dry run installs nothing: the CRDs, setup and Inrun the
	// expectations need must already be in the cluster.
	if r.dryRun {
		fmt.Println("→ Dry run (server): skipping CRDs, setup and the Inrun install — they must already be in the cluster")
		return result, nil
	}

	fmt.Println("→ Ensuring dependencies...")
	if err := cluster.EnsureDependencies(); err != nil {
		return nil, fmt.Errorf("dependencies: %w", err)
	}

	// From here on, an error returns result too: the caller tears down what
	// was applied before the failure.
	crdPaths, err := r.applyCRD(ctx)
	result.appliedCRDPaths = crdPaths
	if err != nil {
		return result, fmt.Errorf("applying CRD: %w", err)
	}

	if r.blueprintFile != "" && !r.kubernetesTarget {
		if err := r.pullOCIImports(ctx); err != nil {
			return result, fmt.Errorf("pulling OCI imports: %w", err)
		}
		bundleFile, err := r.generateBundle(ctx)
		if err != nil {
			return result, fmt.Errorf("generate bundle: %w", err)
		}
		if ownsCluster {
			defer os.Remove(bundleFile)
		} else {
			result.appliedBundlePath = bundleFile
		}
		fmt.Println("→ Applying bundle...")
		if out, err := kubectl.Apply(ctx, kubectl.ApplyOptions{File: bundleFile}); err != nil {
			return result, fmt.Errorf("apply bundle: %w\n%s", err, out)
		}
		fmt.Printf("  %s Bundle applied\n", successMark())
	}

	setupPaths, err := r.applySetup(ctx)
	result.appliedSetupPaths = setupPaths
	if err != nil {
		return result, fmt.Errorf("setup: %w", err)
	}

	if r.devServer {
		devManifest, err := applyDevServer(ctx)
		if err != nil {
			return result, fmt.Errorf("dev server: %w", err)
		}
		result.appliedSetupPaths = append(result.appliedSetupPaths, devManifest)
		fmt.Println("→ Waiting for dev server to be ready...")
		if err := checkDevServerHealth(); err != nil {
			return result, err
		}
		fmt.Printf("  %s Dev server ready\n", successMark())
	}

	// Install Inrun
	if !r.kubernetesTarget {
		if err := r.installInrun(); err != nil {
			return result, err
		}
		result.installedInrun = true
	}

	return result, nil
}
