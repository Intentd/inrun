package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/tools/cluster"
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

// loadImagesIntoCluster resolves the images this run should load — the
// schema's setup.images and the --images flag — and loads them into
// the kind cluster. It is a no-op when the run has no cluster or no images.
func (r *Runner) loadImagesIntoCluster(ctx context.Context) error {
	if !r.needsCluster {
		return nil
	}
	setup := r.e2e.Spec.Setup
	if setup == nil {
		return nil
	}

	declared := setup.Images.ResolveDefaultNames(r.e2e.Metadata.Name)
	allowed, err := setup.EffectiveLoadImages(r.e2e.Metadata.Name, r.images)
	if err != nil {
		return err
	}
	if len(allowed) == 0 {
		return nil
	}

	// Keep only the entries the CLI selected, so a narrow --images
	// skips the builds it did not ask for.
	selected := make(cluster.Images, 0, len(allowed))
	set := make(map[string]struct{}, len(allowed))
	for _, ref := range allowed {
		set[ref] = struct{}{}
	}
	for _, img := range declared {
		if _, ok := set[img.Image]; ok {
			selected = append(selected, img)
		}
	}

	name, ok := r.kindClusterName(ctx)
	if !ok {
		return fmt.Errorf(
			"loading images requires a kind cluster; current context %q is not kind",
			r.effectiveContext(ctx),
		)
	}
	return cluster.LoadImages(ctx, name, selected)
}
