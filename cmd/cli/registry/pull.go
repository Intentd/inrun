//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"os"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/module"
	"github.com/intentd/inrun/pkg/utils"
	"github.com/spf13/cobra"
)

// ── pull ──────────────────────────────────────────────────────────────────────

var pullCmd = &cobra.Command{
	Use:   "pull [<name>:<version>]",
	Short: "Pull a pattern to the local cache",
	Args:  cobra.RangeArgs(0, 1),
	Example: `  inrun pull postgres:v14
  inrun pull oci://ghcr.io/myorg/patterns/redis:v7
  inrun pull -f catalog.yaml
  inrun pull -f stack.yaml
  inrun pull postgres:v14 --refresh`,
	RunE: func(cmd *cobra.Command, args []string) error {
		refresh, _ := cmd.Flags().GetBool("refresh")
		outDir, _ := cmd.Flags().GetString("out")
		filePath, _ := cmd.Flags().GetString("file")

		if filePath != "" {
			return pullFromFile(cmd, filePath, refresh)
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a reference (e.g. postgres:v14) or --file <catalog.yaml>")
		}

		isModule, _ := cmd.Flags().GetBool("module")
		kind := registry.CatalogKind
		if isModule {
			kind = registry.ModuleKind
		}
		ref, err := registry.ResolveForKind(args[0], kind)
		if err != nil {
			return fmt.Errorf("invalid reference: %w", err)
		}

		client, err := registry.NewClient()
		if err != nil {
			return fmt.Errorf("initializing client: %w", err)
		}

		if ref.IsCached() && !refresh {
			cacheDir, _ := ref.CachePath()
			fmt.Printf("  %s Already cached\n", cmdutil.SuccessMark())
			fmt.Printf("  → %s\n", cacheDir)
			printPullSuggestions(ref, cacheDir)
			return nil
		}

		fmt.Printf("Pulling %s\n  → %s\n", ref.ShortName(), ref.String())
		spin := cmdutil.StartSpinner("Downloading...")
		cacheDir, err := client.Pull(cmd.Context(), ref, refresh)
		if err != nil {
			spin.Failure()
			return fmt.Errorf("pull failed: %w", err)
		}
		spin.Stop()

		if outDir != "" {
			if err := cmdutil.CopyDir(cacheDir, outDir); err != nil {
				return fmt.Errorf("extracting to %s: %w", outDir, err)
			}
			fmt.Printf("  %s Extracted to %s\n", cmdutil.SuccessMark(), outDir)
			if spec, err := registry.SpecFor(kind); err == nil {
				if meta, err := registry.LoadPatternMeta(cacheDir, spec); err == nil && meta.Deprecated != nil {
					cmdutil.PrintPatternDeprecation(meta.Deprecated)
				}
			}
			return nil
		}

		fmt.Printf("  %s Cached at %s\n", cmdutil.SuccessMark(), cacheDir)
		printPullSuggestions(ref, cacheDir)

		if !isModule {
			pullModuleDeps(cacheDir)
			notifyTypedPull(cacheDir)
		}

		if spec, err := registry.SpecFor(kind); err == nil {
			if meta, err := registry.LoadPatternMeta(cacheDir, spec); err == nil && meta.Deprecated != nil {
				cmdutil.PrintPatternDeprecation(meta.Deprecated)
			}
		}

		return nil
	},
}

func init() {
	pullCmd.Flags().Bool("refresh", false, "Bypass local cache and re-pull from registry")
	pullCmd.Flags().StringP("out", "o", "", "Extract pulled pattern to this directory")
	pullCmd.Flags().StringP("file", "f", "", "Pull all OCI imports from a catalog or stack file")
	pullCmd.Flags().BoolP("module", "m", false, "Resolve as a module (uses INRUN_MODULES_REGISTRY)")
	cmdutil.RootCmd.AddCommand(pullCmd)

	// Shadow global flags so they don't appear under `inrun pull`
	cmdutil.ShadowGlobalCommandFlags(pullCmd)
}

// notifyTypedPull prints a build note when the pulled artifact is a typed operator.
func notifyTypedPull(cacheDir string) {
	if _, err := os.Stat(cmdutil.JoinPath(cacheDir, registry.FileGoMod)); err != nil {
		return
	}
	_, hasMakefile := os.Stat(cmdutil.JoinPath(cacheDir, registry.FileMakefile))
	fmt.Printf("  ↳ Typed operator — requires a custom runtime\n")
	cmdutil.PrintTypedBuildSteps(hasMakefile == nil)
}

// pullModuleDeps reads the catalog.yaml in cacheDir and pulls any OCI module
// imports it declares. Warnings are printed but do not fail the main pull.
func pullModuleDeps(catalogCacheDir string) {
	catalogFile := cmdutil.JoinPath(catalogCacheDir, registry.FileCatalog)
	if _, err := os.Stat(catalogFile); err != nil {
		return
	}

	imports, err := registry.ExtractOCIImports(catalogFile)
	if err != nil || len(imports.ModuleImports) == 0 {
		return
	}

	if imports.Empty() {
		return
	}

	fmt.Printf("\nPulling module dependencies...\n")
	for _, imp := range imports.ModuleImports {
		spin := cmdutil.StartSpinner(imp.Module)
		if module.PullImport(&imp) == nil {
			spin.Stop()
			fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), imp.Module)
		} else {
			spin.Failure()
			fmt.Printf("  %s %s (pull failed — will retry on next use)\n", cmdutil.WarningMark(), imp.Module)
		}
	}
}

// pullFromFile extracts all OCI refs from a catalog or stack file and pulls
// each one. Module imports (Catalog) and registry imports (Stack) are both
// handled, including bare-name shorthands without an oci:// prefix.
func pullFromFile(cmd *cobra.Command, filePath string, refresh bool) error {
	imports, err := registry.ExtractOCIImports(filePath)
	if err != nil {
		return fmt.Errorf("reading imports from %s: %w", filePath, err)
	}
	if imports.Empty() {
		fmt.Printf("  %s No OCI imports found in %s\n", cmdutil.SuccessMark(), filePath)
		return nil
	}

	client, err := registry.NewClient()
	if err != nil {
		return fmt.Errorf("initializing client: %w", err)
	}

	var errs []string

	for _, imp := range imports.ModuleImports {
		fmt.Printf("Pulling module %s...\n", imp.Module)
		if err := module.PullImport(&imp); err != nil {
			fmt.Printf("  %s %v\n", cmdutil.FailureMark(), err)
			errs = append(errs, err.Error())
		} else {
			fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), imp.Module)
		}
	}

	for _, src := range imports.RegistrySources {
		cleanURL, version := src.ResolvedURL()
		cleanURL = strings.TrimPrefix(cleanURL, "oci://")
		ref, err := registry.Resolve(cleanURL + ":" + version)
		if err != nil {
			fmt.Printf("  %s resolving %s: %v\n", cmdutil.FailureMark(), src.URL, err)
			errs = append(errs, err.Error())
			continue
		}
		if ref.IsCached() && !refresh {
			cacheDir, _ := ref.CachePath()
			fmt.Printf("  %s Already cached: %s\n", cmdutil.SuccessMark(), ref.ShortName())
			pullModuleDeps(cacheDir)
			continue
		}
		fmt.Printf("Pulling %s\n  → %s\n", ref.ShortName(), ref.String())
		spinRef := cmdutil.StartSpinner("Downloading...")
		cacheDir, err := client.Pull(cmd.Context(), ref, refresh)
		if err != nil {
			spinRef.Failure()
			errs = append(errs, err.Error())
		} else {
			spinRef.Stop()
			fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), ref.ShortName())
			pullModuleDeps(cacheDir)
		}
	}

	// ── Helm sources ─────────────────────────────────────────────────────────
	helmImports, err := registry.ExtractHelmAndFileImports(filePath)
	if err == nil && !helmImports.Empty() {
		for _, src := range helmImports.HelmSources {
			label := src.Repo + "/" + src.Chart + "@" + src.Version
			spin := cmdutil.StartSpinner(label)
			if cacheErr := merger.WarmHelmSource(src, refresh); cacheErr != nil {
				spin.Failure()
				errs = append(errs, fmt.Sprintf("helm %s: %v", label, cacheErr))
			} else {
				spin.Stop()
				fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), label)
			}
		}

		// ── Remote file sources ───────────────────────────────────────────────
		for _, url := range helmImports.RemoteFiles {
			spin := cmdutil.StartSpinner(url)
			if refresh {
				utils.InvalidateFileCache(url)
			}
			if _, fetchErr := utils.LoadFileWithAuthRefresh(url, nil, refresh); fetchErr != nil {
				spin.Failure()
				errs = append(errs, fmt.Sprintf("file %s: %v", url, fetchErr))
			} else {
				spin.Stop()
				fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), url)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d pull(s) failed:\n  %s", len(errs), strings.Join(errs, "\n  "))
	}
	return nil
}
