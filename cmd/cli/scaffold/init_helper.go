//go:build !runtime && !gateway

package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/examples"
	toolsscaffold "github.com/intentd/tools/scaffold"
	"github.com/intentd/tools/version"
)

// extractEmbeddedPack extracts a pack bundled with Inrun into the project.
func extractEmbeddedPack(root, pack string) error {
	p, ok := GetPack(pack)
	if !ok {
		return fmt.Errorf("unknown pack %q", pack)
	}

	return toolsscaffold.ExtractPack(
		root,
		examples.FS,
		toolsscaffold.Pack{
			Name:  p.Name,
			Path:  p.Path,
			First: p.First,
		},
		toolsscaffold.Options{
			GoModReplacements: map[string]string{
				"{{ .InrunVersion }}": version.Short(),
			},
		},
	)
}

// extractCanonical extracts the canonical hello-website operator files into
// the project's standard directory layout.
func extractCanonical(root string) error {
	return toolsscaffold.ExtractCanonical(
		root,
		examples.FS,
		"reconcilers/declarative/01-hello-website",
		[]toolsscaffold.CanonicalFile{
			{Source: "blueprint.yaml", Destination: "blueprint.yaml"},
			{Source: "manifests/crd.yaml", Destination: "manifests/crd.yaml"},
			{Source: "manifests/cr.yaml", Destination: "manifests/cr.yaml"},
			{Source: "test/simulate.yaml", Destination: "test/simulate.yaml"},
			{Source: "test/e2e.yaml", Destination: "test/e2e.yaml"},
		},
	)
}

// downloadExamplePack downloads a release archive or returns its cached path.
func downloadExamplePack(pack, version string, refresh bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return toolsscaffold.DownloadPack(pack, toolsscaffold.DownloadOptions{
		Repository: "intentd/inrun",
		Version:    version,
		CacheDir:   filepath.Join(home, ".inrun", "packs"),
		Refresh:    refresh,
	})
}

// extractExamplePack extracts a downloaded pack archive into the project.
func extractExamplePack(root, pack, archivePath string) error {
	return toolsscaffold.ExtractPackArchive(root, pack, archivePath)
}

// clearCache removes all cached Inrun pack archives.
func clearCache() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	return toolsscaffold.ClearCache(filepath.Join(home, ".inrun", "packs"))
}

type initStep struct {
	name string
	fn   func() error
}

// runSteps executes initialization steps and prints their results.
func runSteps(steps []initStep) error {
	for _, step := range steps {
		fmt.Printf("  %-50s", step.name+"...")
		if err := step.fn(); err != nil {
			fmt.Printf("%s\n", cmdutil.FailureMark())
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Printf("%s\n", cmdutil.SuccessMark())
	}

	return nil
}

// isCurrentDirectory reports whether the project name refers to the current
// working directory.
func isCurrentDirectory(name string) bool {
	return name == "."
}
