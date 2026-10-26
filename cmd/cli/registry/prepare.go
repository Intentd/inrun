//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/spf13/cobra"
)

// resolvePushVersion reconciles the requested reference with metadata and persists an explicitly requested metadata override.
func resolvePushVersion(
	cmd *cobra.Command,
	dir, primaryFile, name, metadataVersion, refArg string,
	force, updateMeta bool,
) (resolvedRef, resolvedVersion string, err error) {
	providedTag := ""
	if refArg != "" {
		providedTag = registry.ExtractTagVersion(refArg)
	}

	if refArg == "" {
		if name == "" {
			return "", metadataVersion, fmt.Errorf("metadata.name is required in %s", primaryFile)
		}
		if metadataVersion == "" {
			metadataVersion = "latest"
		}
		refArg = fmt.Sprintf("%s:%s", name, metadataVersion)
		providedTag = metadataVersion
	} else {
		if providedTag != "" && metadataVersion != "" && metadataVersion != providedTag {
			msg := fmt.Errorf("%s: metadata.version %q does not match provided tag %q; use '--force' to override", primaryFile, metadataVersion, providedTag)
			if !force {
				return "", metadataVersion, msg
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v (continuing due to --force)\n", cmdutil.Yellow("Warning"), msg)
			if updateMeta {
				if err := registry.PersistMetadataVersion(dir, primaryFile, providedTag); err != nil {
					return "", metadataVersion, fmt.Errorf("failed to update metadata in %s: %w", primaryFile, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "updated %s: metadata.version -> %q\n", primaryFile, providedTag)
			}
			metadataVersion = providedTag
		}
		if metadataVersion == "" && providedTag != "" {
			metadataVersion = providedTag
		}
	}
	return refArg, metadataVersion, nil
}

// preparePushFiles validates files that need semantic validation before gates
// run, then prints the files that will be included in the artifact.
func preparePushFiles(dir string, isBlueprint bool, files []string) error {
	if isBlueprint {
		localImports, err := registry.ExtractLocalModuleImports(filepath.Join(dir, registry.FileBlueprint))
		if err == nil && len(localImports) > 0 {
			var lines []string
			for _, li := range localImports {
				lines = append(lines, fmt.Sprintf("  spec.crds.%s.imports[%d]: %q", li.CRDName, li.Index, li.Path))
			}
			return fmt.Errorf(
				"✗ Push blocked: local file imports in %s\n\n%s\n\n"+
					"  Local imports work for inrun simulate and inrun template, but cannot\n"+
					"  be resolved by consumers after the blueprint is published.\n\n"+
					"  Before publishing:\n"+
					"    1. Push the module:  inrun push <module-dir>/\n"+
					"    2. Replace the local path with the OCI ref:\n"+
					"       module: oci://<your-registry>/modules/<name>:<version>",
				registry.FileBlueprint,
				strings.Join(lines, "\n"),
			)
		}

		m := merger.New(filepath.Join(dir, registry.FileBlueprint))
		if err := m.Merge(); err != nil {
			return fmt.Errorf("  ✗ %s: %w", registry.FileBlueprint, err)
		}
		bp, err := pipeline.BuildExpanded(cmdutil.Cfg, m)
		if err != nil {
			return fmt.Errorf("  ✗ %s: %w", registry.FileBlueprint, err)
		}
		fmt.Printf("  %s %-20s valid\n", cmdutil.SuccessMark(), registry.FileBlueprint)
		if d := bp.Deprecation(); d != nil {
			cmdutil.PrintBlueprintDeprecation(d)
		}

		if crd := registry.FindPatternFile(dir, registry.FileCRD); crd != "" && containsFile(files, crd) {
			if err := cmdutil.ValidateCRDFile(filepath.Join(dir, crd)); err != nil {
				return fmt.Errorf("  ✗ %s: %w", crd, err)
			}
			fmt.Printf("  %s %-20s valid\n", cmdutil.SuccessMark(), crd)
		}
	}

	for _, f := range files {
		if f == registry.FileBlueprint || filepath.Base(f) == registry.FileCRD {
			continue
		}
		info, _ := os.Stat(filepath.Join(dir, f))
		if info != nil {
			fmt.Printf("  %s %-20s (%s)\n", cmdutil.SuccessMark(), f, cmdutil.FormatSize(info.Size()))
		}
	}
	return nil
}

func containsFile(files []string, target string) bool {
	for _, file := range files {
		if file == target {
			return true
		}
	}
	return false
}
