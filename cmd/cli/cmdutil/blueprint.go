//go:build !runtime && !gateway

package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

// DefaultNamespace returns the namespace to use when --namespace is not supplied.
// Reads INRUN_NAMESPACE from the environment so that CLI invocations inside
// an already-configured cluster automatically target the right namespace.
func DefaultNamespace() string {
	if ns := os.Getenv("INRUN_NAMESPACE"); ns != "" {
		return ns
	}
	return "inrun-system"
}

// ParseFilePaths handles comma-separated values and returns a slice of paths
func ParseFilePaths(paths []string) []string {
	var expanded []string
	for _, p := range paths {
		// Split by comma and trim spaces
		parts := strings.Split(p, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				expanded = append(expanded, trimmed)
			}
		}
	}
	return expanded
}

type MergerOut struct {
	Merger    *merger.Merger
	CRDs      []types.CRDEntry
	Blueprint *blueprint.Blueprint
	Paths     []string
	Enabled   map[string]types.CRDEntry // m.Enabled()
}

func GenerateBlueprint(cmd *cobra.Command) (*MergerOut, error) {
	blueprintPaths, _ := cmd.Flags().GetStringSlice("file")

	expanded := ParseFilePaths(blueprintPaths)
	if len(expanded) == 0 {
		expanded = DefaultFilePaths()
	}
	if len(expanded) == 0 {
		return nil, fmt.Errorf(ErrNoBlueprint)
	}

	// Absolutize so merger.FirstEntryDir() is always absolute. Without this,
	// relative paths cause a double-join when PopulateAPITypesFromCRDFile
	// prepends blueprintDir to a crdFile that was already joined once during load.
	for i, p := range expanded {
		if abs, err := filepath.Abs(p); err == nil {
			expanded[i] = abs
		}
	}

	m := merger.New(expanded...)
	if err := m.Merge(); err != nil {
		return nil, err
	}

	var bp blueprint.Blueprint
	bp.Spec = m.ToSpec()

	// Fill apiTypes from crdFile, as the pipeline does, so generators see a
	// fully specified CRD. The merger has already made crdFile absolute.
	crdMap := bp.Spec.CRDs
	if err := resolveCRDFiles(crdMap); err != nil {
		return nil, err
	}
	enabled := m.Enabled()
	if err := resolveCRDFiles(enabled); err != nil {
		return nil, err
	}

	// Convert map to slice for generate functions
	crds := make([]types.CRDEntry, 0, len(crdMap))
	for _, c := range crdMap {
		crds = append(crds, c)
	}

	return &MergerOut{
		Merger:    m,
		CRDs:      crds,
		Blueprint: &bp,
		Paths:     blueprintPaths,
		Enabled:   enabled,
	}, nil
}

// resolveCRDFiles fills each entry's apiTypes from its crdFile and clears
// crdFile, so nothing downstream depends on a local path.
func resolveCRDFiles(crds map[string]types.CRDEntry) error {
	for name, crd := range crds {
		if crd.CRDFile == "" {
			continue
		}
		if err := blueprint.PopulateAPITypesFromCRDFile(&crd, ""); err != nil {
			return fmt.Errorf("CRD %q: %w", name, err)
		}
		crd.CRDFile = ""
		crds[name] = crd
	}
	return nil
}

// BuildBlueprint builds the expanded Blueprint using the --file flag from cmd.
func BuildBlueprint(cmd *cobra.Command) (*blueprint.Blueprint, error) {
	m, err := GenerateBlueprint(cmd)
	if err != nil {
		return nil, fmt.Errorf("generating Blueprint: %w", err)
	}
	return pipeline.BuildExpanded(Cfg, m.Merger)
}
