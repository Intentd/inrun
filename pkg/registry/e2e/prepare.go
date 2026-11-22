package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/tools/cluster"
)

// ensureDependencies expands expectation includes, checks required tools,
// and prepares the cluster dependencies needed by the run.
//
// It skips cluster setup for commands-only runs and server dry runs. Any
// resources applied before an error are recorded in dependencyResult so the
// caller can clean them up.
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

	setupPaths, err := r.applyPrepare(ctx)
	result.appliedPreparePaths = setupPaths
	if err != nil {
		return result, fmt.Errorf("prepare: %w", err)
	}

	if r.devServer {
		devManifest, err := applyDevServer(ctx)
		if err != nil {
			return result, fmt.Errorf("dev server: %w", err)
		}

		result.appliedPreparePaths = append(result.appliedPreparePaths, devManifest)

		fmt.Println("→ Waiting for dev server to be ready...")
		if err := checkDevServerHealth(); err != nil {
			return result, err
		}
		fmt.Printf("  %s Dev server ready\n", successMark())
	}

	// Install Inrun.
	if !r.kubernetesTarget {
		if err := r.installInrun(); err != nil {
			return result, err
		}
		result.installedInrun = true
	}

	return result, nil
}

// prepareImages loads and deploys the images selected for this run.
//
// Images are loaded before their Deployments are applied so that locally
// built images are available to the cluster when the pods are scheduled.
func (r *Runner) prepareImages(ctx context.Context) error {
	if err := r.loadImagesIntoCluster(ctx); err != nil {
		return err
	}

	if err := r.deployImagesToCluster(ctx); err != nil {
		return err
	}

	return nil
}

// loadImagesIntoCluster resolves the images this run should load and loads them
// into the cluster.
//
// It is a no-op when the run does not need a cluster or has no images to
// load. A CLI --images selection limits which declared images are loaded.
func (r *Runner) loadImagesIntoCluster(ctx context.Context) error {
	if !r.needsCluster {
		return nil
	}

	env := r.e2e.Spec.Environment
	if env == nil {
		return nil
	}

	images, err := env.EffectiveLoadImages(r.e2e.Metadata.Name, r.images)
	if err != nil {
		return err
	}

	if len(images) == 0 {
		return nil
	}

	// Resolve the selected references back to their full image definitions.
	// This preserves build configuration for cluster.LoadImages.
	// Build paths resolve like Compose: context against the spec's directory,
	// dockerfile against the context.
	declared := env.Images.ResolveDefaultNames(r.e2e.Metadata.Name).ResolvePaths(r.e2eDir)

	set := make(map[string]struct{}, len(images))
	for _, ref := range images {
		set[ref] = struct{}{}
	}

	selected := make(cluster.Images, 0, len(images))
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

// deployImagesToCluster applies the Deployments and Services configured for
// the images selected for this run.
//
// It is a no-op when the run does not need a cluster or has no images to
// deploy. A CLI --images selection limits which declared images are deployed.
func (r *Runner) deployImagesToCluster(ctx context.Context) error {
	if !r.needsCluster {
		return nil
	}

	env := r.e2e.Spec.Environment
	if env == nil {
		return nil
	}

	images, err := env.EffectiveDeployImages(r.e2e.Metadata.Name, r.images)
	if err != nil {
		return err
	}

	if len(images) == 0 {
		return nil
	}
	return cluster.DeployImages(ctx, r.cs, images, cluster.WithFieldManager("inrun-e2e"), cluster.WithForceOwnership())
}
