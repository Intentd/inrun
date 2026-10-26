package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
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
	result.expects, err = expect.ExpandIncludes(r.e2e.Spec.Expect, r.e2eDir)
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

	fmt.Println("→ Ensuring dependencies...")
	if err := cluster.EnsureDependencies(); err != nil {
		return nil, fmt.Errorf("dependencies: %w", err)
	}

	crdPaths, err := r.applyCRD(ctx)
	if err != nil {
		return nil, fmt.Errorf("applying CRD: %w", err)
	}
	result.appliedCRDPaths = crdPaths

	if r.blueprintFile != "" && !r.kubernetesTarget {
		if err := r.pullOCIImports(ctx); err != nil {
			return nil, fmt.Errorf("pulling OCI imports: %w", err)
		}
		bundleFile, err := r.generateBundle(ctx)
		if err != nil {
			return nil, fmt.Errorf("generate bundle: %w", err)
		}
		if ownsCluster {
			defer os.Remove(bundleFile)
		} else {
			result.appliedBundlePath = bundleFile
		}
		fmt.Println("→ Applying bundle...")
		if out, err := runKubectl(ctx, "", types.NoopEnvResolver(), "apply", "-f", bundleFile); err != nil {
			return nil, fmt.Errorf("apply bundle: %w\n%s", err, out)
		}
		fmt.Printf("  %s Bundle applied\n", successMark())
	}

	setupPaths, err := r.applySetup(ctx)
	if err != nil {
		return nil, fmt.Errorf("setup: %w", err)
	}
	result.appliedSetupPaths = setupPaths

	if r.devServer {
		devManifest, err := applyDevServer(ctx)
		if err != nil {
			return nil, fmt.Errorf("dev server: %w", err)
		}
		result.appliedSetupPaths = append(result.appliedSetupPaths, devManifest)
		fmt.Println("→ Waiting for dev server to be ready...")
		if err := checkDevServerHealth(); err != nil {
			return nil, err
		}
		fmt.Printf("  %s Dev server ready\n", successMark())
	}

	// Install Inrun
	if !r.kubernetesTarget {
		if err := r.installInrun(); err != nil {
			return nil, err
		}
		result.installedInrun = true
	}

	return result, nil
}
