//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
)

// detectCacheType resolves the artifact kind by checking for sentinel files on
// disk — not by sniffing the path string. Returns (isModule, isBlueprint, patternFile).
func detectCacheType(cacheDir string) (bool, bool, string) {
	moduleFile := filepath.Join(cacheDir, registry.FileModule)
	if _, err := os.Stat(moduleFile); err == nil {
		return true, false, moduleFile
	}
	blueprintFile := filepath.Join(cacheDir, registry.FileBlueprint)
	if _, err := os.Stat(blueprintFile); err == nil {
		return false, true, blueprintFile
	}
	return false, false, ""
}

// readModule reads and unmarshals a module file. Returns nil and error on failure.
func readModule(path string) (*types.Module, error) {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return nil, err
	}
	var m types.Module
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// inputsSummary returns derived values and a small sample of inputs to print.
type inputsSummary struct {
	TotalCount       int
	HasInputs        bool
	RequiredCount    int
	RequiredInputs   []types.ModuleInput
	FirstTwoDefaults []types.ModuleInput
}

func collectInputsSummary(inputs []types.ModuleInput) inputsSummary {
	s := inputsSummary{TotalCount: len(inputs)}
	if s.TotalCount == 0 {
		return s
	}
	s.HasInputs = true
	for _, in := range inputs {
		if in.Required {
			s.RequiredInputs = append(s.RequiredInputs, in)
		}
	}
	s.RequiredCount = len(s.RequiredInputs)
	// collect first two non-required inputs for display
	for _, in := range inputs {
		if !in.Required && len(s.FirstTwoDefaults) < 2 {
			s.FirstTwoDefaults = append(s.FirstTwoDefaults, in)
		}
	}
	return s
}

// printValidationHint prints the inrun validate hint for the pattern file.
func printValidationHint(patternFile string) {
	if patternFile == "" {
		return
	}
	fmt.Print("\nValidate the pattern:\n")
	fmt.Printf("  inrun validate -f %s\n", patternFile)
}

// printModuleReference prints module usage and inputs sample.
func printModuleReference(ref *registry.Ref, module *types.Module, sum inputsSummary) {
	fmt.Printf("\nReference in a Blueprint:\n")
	fmt.Printf("  imports:\n")
	fmt.Printf("    - module: %s\n", ref.String())

	if !sum.HasInputs {
		return
	}

	fmt.Printf("      with:\n")
	shown := 0
	if sum.RequiredCount > 0 {
		for i := 0; i < sum.RequiredCount && i < 2; i++ {
			fmt.Printf("        %-30s # required\n", sum.RequiredInputs[i].Name+": <value>")
			shown++
		}
	}
	for _, in := range sum.FirstTwoDefaults {
		if shown >= 2 {
			break
		}
		fmt.Printf("        %-30s # defaults to: %s\n", in.Name+": <value>", in.Default)
		shown++
	}
	remaining := sum.TotalCount - shown
	if remaining > 0 {
		fmt.Printf("        # ...and %d more inputs\n", remaining)
	}
}

// printBlueprintReference prints blueprint usage and run hint.
func printBlueprintReference(ref *registry.Ref, cacheDir string) {
	fmt.Printf("\nRun this blueprint pattern:\n")
	fmt.Printf("  inrun -f %s\n", filepath.Join(cacheDir, registry.FileBlueprint))
	fmt.Printf("\nOr reference in a Stack:\n")
	fmt.Printf("  imports:\n")
	fmt.Printf("    registry:\n")
	fmt.Printf("      - url: %s\n", ref.String())
}

// printCachedFiles lists the files in cacheDir so the user knows what was pulled.
func printCachedFiles(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}
	fmt.Printf("\n  Files:\n")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		fmt.Printf("    %-30s %s\n", e.Name(), cmdutil.FormatSize(size))
	}
}

// printPullSuggestions orchestrates the helpers and handles errors gracefully.
func printPullSuggestions(ref *registry.Ref, cacheDir string) {
	isModule, isBlueprint, patternFile := detectCacheType(cacheDir)
	printCachedFiles(cacheDir)
	printValidationHint(patternFile)

	if isModule {
		module, err := readModule(patternFile)
		if err != nil {
			// non-fatal: print a short message and return
			fmt.Fprintf(os.Stderr, "warning: failed to read module %s: %v\n", patternFile, err)
			return
		}
		sum := collectInputsSummary(module.Inputs)
		printModuleReference(ref, module, sum)
		return
	}

	if isBlueprint {
		printBlueprintReference(ref, cacheDir)
		return
	}

	// fallback: nothing recognized
	fmt.Println("No module or blueprint pattern detected in cache.")
}
