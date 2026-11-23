package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/module"
)

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
