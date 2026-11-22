package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/cluster"
	"gopkg.in/yaml.v3"
)

// applyCRD applies the operator's CRD to the cluster and returns the paths applied.
// Uses spec.AllCRDPaths() if declared; falls back to crdFile entries in the blueprint.
func (r *Runner) applyCRD(ctx context.Context) ([]string, error) {
	if crdPaths := r.e2e.Spec.AllCRDPaths(); len(crdPaths) > 0 {
		var applied []string
		for _, crd := range crdPaths {
			path := r.abs(crd)
			fmt.Printf("→ Applying CRD from %s...\n", crd)
			if out, err := kubectl.Apply(ctx, kubectl.ApplyOptions{File: path}); err != nil {
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
				Migrate struct {
					Transitional struct {
						CRDFile string `yaml:"crdFile"`
					} `yaml:"transitional"`
				} `yaml:"migrate"`
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
		// A declared transitional CRD holds the old and new paths, so the
		// gateway's migration can move CRs: it is applied in crdFile's place.
		file := entry.CRDFile
		if t := entry.Migrate.Transitional.CRDFile; t != "" {
			file = t
		}
		if file == "" {
			continue
		}
		path := file
		if !filepath.IsAbs(path) && !strings.HasPrefix(path, "http") {
			path = filepath.Join(blueprintDir, path)
		}
		fmt.Printf("→ Applying CRD '%s' from %s...\n", name, file)
		if out, err := kubectl.Apply(ctx, kubectl.ApplyOptions{File: path}); err != nil {
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

// applyEnvironment provisions the environment phase: apply, helm and wait
// for cluster furniture, after the cluster exists and images are loaded.
func (r *Runner) applyEnvironment(ctx context.Context) ([]string, error) {
	env := r.e2e.Spec.Environment
	if env == nil {
		return nil, nil
	}
	return r.applyProvision(ctx, "environment", env.Provision)
}

// applyPrepare provisions the prepare phase: ecosystem dependencies, then
// apply, helm and wait for the workload.
func (r *Runner) applyPrepare(ctx context.Context) ([]string, error) {
	p := r.e2e.Spec.Prepare
	if p == nil {
		return nil, nil
	}
	applied, err := r.applyProvision(ctx, "prepare", p.Provision)
	if err != nil {
		return applied, err
	}
	return applied, nil
}

// applyProvision runs the three provision sub-phases in order — apply, helm,
// wait — for one phase. The phase name appears in every message so a failure
// says which phase and which step it failed in.
func (r *Runner) applyProvision(ctx context.Context, phase string, p types.Provision) ([]string, error) {
	var applied []string

	// ── Phase 1: apply ─────────────────────
	for _, entry := range p.Apply {
		abs := r.abs(entry.Path)
		fmt.Printf("→ Applying %s %s...\n", phase, entry.Path)
		if out, err := kubectl.Apply(ctx, kubectl.ApplyOptions{File: abs}); err != nil {
			return applied, fmt.Errorf("%s apply %s: %w\n%s", phase, entry.Path, err, out)
		}
		fmt.Printf("  %s Applied\n", successMark())
		applied = append(applied, abs)
		for _, w := range entry.Wait {
			if err := runProvisionWait(ctx, w); err != nil {
				return applied, fmt.Errorf("%s apply %s wait: %w", phase, entry.Path, err)
			}
		}
	}

	// ── Phase 2: helm ──────────────────────
	for _, h := range p.Helm {
		if h.IsLocalChart() && !filepath.IsAbs(h.Chart) {
			h.Chart = r.abs(h.Chart)
		}
		for i, f := range h.ValueFiles {
			if f != "" && !filepath.IsAbs(f) {
				h.ValueFiles[i] = r.abs(f)
			}
		}
		sp := startSpinner(fmt.Sprintf("Installing %s %s...", phase, h.ReleaseName()))
		if err := cluster.HelmInstall(ctx, h.HelmRelease()); err != nil {
			sp.Failure()
			return applied, fmt.Errorf("%s helm %s: %w", phase, h.Chart, err)
		}
		sp.Success()
		for _, w := range h.Wait {
			if err := runProvisionWait(ctx, w); err != nil {
				return applied, fmt.Errorf("%s helm %s wait: %w", phase, h.ReleaseName(), err)
			}
		}
	}

	// ── Phase 3: wait ──────────────────────
	for _, w := range p.Wait {
		if err := runProvisionWait(ctx, w); err != nil {
			return applied, fmt.Errorf("%s wait: %w", phase, err)
		}
	}

	return applied, nil
}

func runProvisionWait(ctx context.Context, w types.ProvisionWait) error {
	loc := w.Kind + " " + w.Name
	if w.Namespace != "" {
		loc += " (" + w.Namespace + ")"
	}
	sp := startSpinner(fmt.Sprintf("Waiting for %s...", loc))
	if err := cluster.WaitForResource(ctx, cluster.Wait(w)); err != nil {
		sp.Failure()
		return err
	}
	sp.Success()
	return nil
}
