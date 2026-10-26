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
	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
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
			if out, err := runKubectl(ctx, "", types.NoopEnvResolver(), "apply", "-f", path); err != nil {
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
		if out, err := runKubectl(ctx, "", types.NoopEnvResolver(), "apply", "-f", path); err != nil {
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

	// ── Phase 1: apply ─────────────────────
	for _, entry := range s.Apply {
		abs := r.abs(entry.Path)
		fmt.Printf("→ Applying setup %s...\n", entry.Path)
		if out, err := runKubectl(ctx, "", types.NoopEnvResolver(), "apply", "-f", abs); err != nil {
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

	// ── Phase 2: helm ──────────────────────
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

	// ── Phase 3: wait ──────────────────────
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
