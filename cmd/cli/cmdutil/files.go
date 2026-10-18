package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/util/fileio"
)

const (
	FileCatalog    = "catalog.yaml"
	fileStack      = "stack.yaml"
	FileE2e        = "e2e.yaml"
	FileSimulate   = "simulate.yaml"
	FileCrd        = "crd.yaml"
	FileCr         = "cr.yaml"
	FileReadMe     = "README.md"
	FileMakeFile   = "Makefile"
	FileDockerfile = "Dockerfile"
	FileValues     = "values.yaml"

	// A project keeps catalog.yaml at its root, what it applies to the cluster
	// (crd.yaml, cr.yaml, setup files) in manifests/, and its simulate.yaml,
	// e2e.yaml and test values in test/.
	DirManifests = "manifests"
	DirTest      = "test"
)

// FindFile returns name if it exists in the current directory, else the
// first dirs/name that exists, else "".
func FindFile(name string, dirs ...string) string {
	if FileExists(name) {
		return name
	}
	for _, d := range dirs {
		if p := filepath.Join(d, name); FileExists(p) {
			return p
		}
	}
	return ""
}

// ResolveCatalogPaths resolves the catalog file paths in the following order:
//
//  1. Explicit CLI paths (highest priority)
//  2. Default file paths in the working directory (catalog.yaml, stack.yaml, etc.)
//  3. Paths defined in the Config (cfg.Catalog().Paths())
//
// Returns an error if no catalog file can be resolved.
func ResolveCatalogPaths(cliPaths []string) ([]string, error) {
	cfgPaths := Cfg.Catalog().Paths()

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

	return nil, fmt.Errorf(ErrNoCatalog)
}

// DefaultFilePaths returns the default catalog file if one exists in the
// current directory and no -f flag was provided. Tries catalog.yaml first,
// then stack.yaml — the same precedence as Docker's Dockerfile / compose.yaml.
func DefaultFilePaths() []string {
	for _, name := range []string{FileCatalog, fileStack} {
		if _, err := os.Stat(name); err == nil {
			return []string{name}
		}
	}
	return nil
}

const ErrNoCatalog = "no catalog.yaml or stack.yaml found in current directory\n" +
	"pass -f <file> or create one with inrun init"

// ResolveCatalogFile resolves a single catalog file path from a CLI flag value.
// If flagValue is empty it falls back to defaultFilePaths(). The resolved path
// is always returned as an absolute path.
func ResolveCatalogFile(flagValue string) (string, error) {
	if flagValue == "" {
		if d := DefaultFilePaths(); len(d) > 0 {
			flagValue = d[0]
		}
	}
	if flagValue == "" {
		return "", fmt.Errorf(ErrNoCatalog)
	}
	if abs, err := filepath.Abs(flagValue); err == nil {
		return abs, nil
	}
	return flagValue, nil
}

// File helpers live in pkg/util/fileio; these names keep existing callers unchanged.
var (
	FileExists = fileio.FileExists
	IsDir      = fileio.IsDir
)
