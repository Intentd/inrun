package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/util/fileio"
)

const (
	FileBlueprint  = registry.FileBlueprint
	fileStack      = registry.FileStack
	FileE2e        = registry.FileE2E
	FileSimulate   = registry.FileSimulate
	FileCrd        = registry.FileCRD
	FileCr         = registry.FileCR
	FileReadMe     = "README.md"
	FileMakeFile   = "Makefile"
	FileDockerfile = "Dockerfile"
	FileValues     = "values.yaml"

	// A project keeps blueprint.yaml at its root, what it applies to the cluster
	// (crd.yaml, cr.yaml, setup files) in manifests/, and its simulate.yaml,
	// e2e.yaml and test values in test/.
	DirManifests = registry.DirManifests
	DirTest      = registry.DirTest
)

// FindFile returns where name is in the project in the current directory:
// its root, else its place in the standard layout (manifests/ or test/), else "".
func FindFile(name string) string {
	return registry.FindPatternFile(".", name)
}

// ResolveBlueprintPaths resolves the blueprint file paths in the following order:
//
//  1. Explicit CLI paths (highest priority)
//  2. Default file paths in the working directory (blueprint.yaml, stack.yaml, etc.)
//  3. Paths defined in the Config (cfg.Blueprint().Paths())
//
// Returns an error if no blueprint file can be resolved.
func ResolveBlueprintPaths(cliPaths []string) ([]string, error) {
	cfgPaths := Cfg.Blueprint().Paths()

	// 1. CLI-provided paths — convert to absolute so all downstream relative
	// path resolutions (crdFile, crFiles, setup, Stack imports) use the
	// file's directory as the base, not the current working directory.
	if len(cliPaths) > 0 {
		abs := make([]string, len(cliPaths))
		for i, p := range cliPaths {
			if a, err := filepath.Abs(p); err == nil {
				abs[i] = a
			} else {
				abs[i] = p
			}
		}
		return abs, nil
	}

	// 2. Default file paths
	if defaults := DefaultFilePaths(); len(defaults) > 0 {
		return defaults, nil
	}

	// 3. Config-defined paths
	if len(cfgPaths) > 0 {
		return cfgPaths, nil
	}

	return nil, fmt.Errorf(ErrNoBlueprint)
}

// DefaultFilePaths returns the default blueprint file if one exists in the
// current directory and no -f flag was provided. Tries blueprint.yaml first,
// then stack.yaml — the same precedence as Docker's Dockerfile / compose.yaml.
func DefaultFilePaths() []string {
	for _, name := range []string{FileBlueprint, fileStack} {
		if _, err := os.Stat(name); err == nil {
			return []string{name}
		}
	}
	return nil
}

const ErrNoBlueprint = "no blueprint.yaml or stack.yaml found in current directory\n" +
	"pass -f <file> or create one with inrun init"

// ResolveBlueprintFile resolves a single blueprint file path from a CLI flag value.
// If flagValue is empty it falls back to defaultFilePaths(). The resolved path
// is always returned as an absolute path.
func ResolveBlueprintFile(flagValue string) (string, error) {
	if flagValue == "" {
		if d := DefaultFilePaths(); len(d) > 0 {
			flagValue = d[0]
		}
	}
	if flagValue == "" {
		return "", fmt.Errorf(ErrNoBlueprint)
	}
	if abs, err := filepath.Abs(flagValue); err == nil {
		return abs, nil
	}
	return flagValue, nil
}

var (
	FileExists = fileio.FileExists
	IsDir      = fileio.IsDir
)

// ChartDir returns a --chart flag value as an absolute path, or "" when it is
// empty. The path must be a Helm chart directory: e2e may change directory
// before installing.
func ChartDir(chart string) (string, error) {
	if chart == "" {
		return "", nil
	}
	abs, err := filepath.Abs(chart)
	if err != nil {
		return "", fmt.Errorf("--chart %q: %w", chart, err)
	}
	if _, err := os.Stat(filepath.Join(abs, "Chart.yaml")); err != nil {
		return "", fmt.Errorf("--chart %q: not a Helm chart directory (no Chart.yaml)", chart)
	}
	return abs, nil
}

// HelmSetArgs turns --set flag values into helm arguments: "--set", value, …
func HelmSetArgs(sets []string) []string {
	var args []string
	for _, s := range sets {
		args = append(args, "--set", s)
	}
	return args
}
