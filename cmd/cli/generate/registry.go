//go:build !runtime && !gateway

package generate

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/tools/generate"
	"github.com/spf13/cobra"
)

// Generate 'pkg/typeregistry/zz_generated_typeregistry.go' for one or more operator projects.
//
// This command scans a Catalog, resolves all CRDs, typed extensions, and
// managed‑resource contracts, and produces a complete runtime registry:
//
//   - CRD registrations
//   - Reconciler registrations (hooks, constructors, operatorBox)
//   - List/watch registrations
//   - Runtime object wiring
//
// The registry is written to:
//
//	pkg/typeregistry/zz_generated_typeregistry.go
//
// A matching cmd/inrun/main.go file is also ensured, containing the required
// blank import for the runtime package.
//
// The command supports both single‑project and multi‑project generation:
//
//   - Single project: run in the project directory (must contain go.mod)
//   - Multi‑project: use --dirs to generate registries for multiple operator
//     projects in one invocation
//
// Examples:
//
//	# Generate registry for the current project
//	inrun generate registry --file catalog.yaml
//
//	# Generate registry for a specific catalog file
//	inrun generate registry --file path/to/catalog.yaml
//
//	# Generate registry for multiple operator projects
//	inrun generate registry --dirs ./website,./database,./pipeline
//
//	# Dry‑run (print generated files without writing them)
//	inrun generate registry --file catalog.yaml --dry-run
//
// Each project directory must contain:
//   - go.mod
//   - a catalog.yaml (or --file pointing to one)
//   - pkg/typeregistry/ (created automatically if missing)
//
// The generated registry is deterministic and reflects the exact typed‑mode
// contracts declared in the Catalog (hooks, constructors, operatorBox).
var generateRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Generate 'pkg/typeregistry/zz_generated_typeregistry.go' from a Catalog",
	Long: `Generate the Inrun runtime registry for one or multiple operator projects.

This command scans a Catalog, resolves all CRDs, typed extensions, and
managed‑resource contracts, and produces the runtime registry used by the
operator process. The registry includes:

  • CRD registrations
  • Reconciler registrations (hooks, constructors, operatorBox)
  • List/watch registrations
  • Runtime object wiring

The registry is written to:
  pkg/typeregistry/zz_generated_typeregistry.go

A matching cmd/inrun/main.go file is also ensured, containing the required
blank import for the runtime package.

You can generate the registry for the current project, or for multiple operator
projects in one invocation using --dirs.

Examples:
  # Generate registry for the current project
  inrun generate registry --file catalog.yaml

  # Generate registry for a specific catalog file
  inrun generate registry --file path/to/catalog.yaml

  # Generate registries for multiple operator projects
  inrun generate registry --dirs ./database,./pipeline --file stack.yaml

  # Dry-run (print generated files without writing them)
  inrun generate registry --file catalog.yaml --dry-run

Each project directory must contain:
  • go.mod
  • a catalog.yaml
  • pkg/typeregistry/ (created automatically if missing)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		dirs, _ := cmd.Flags().GetString("dirs")
		fetchTimeout, _ := cmd.Flags().GetDuration("fetch-timeout")

		// Multi-directory mode
		if dirs != "" {
			for _, d := range strings.Split(dirs, ",") {
				abs := filepath.Clean(d)
				if err := generateRegistryForDir(abs, cmd, fetchTimeout, dryRun); err != nil {
					return fmt.Errorf("registry generation failed for %s: %w", abs, err)
				}
			}
			return nil
		}

		// Single-directory mode (current working directory)
		cwd, _ := os.Getwd()
		return generateRegistryForDir(cwd, cmd, fetchTimeout, dryRun)
	},
}

// generateRegistryForDir performs the full registry generation pipeline
// for a single operator project directory.
func generateRegistryForDir(dir string, cmd *cobra.Command, perModuleTimeout time.Duration, dryRun bool) error {
	// Save original working directory
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	// Enter target project directory
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("chdir %s: %w", dir, err)
	}

	// Load catalog (from --file or default)
	out, err := cmdutil.GenerateCatalog(cmd)
	if err != nil {
		return fmt.Errorf("loading catalog: %w", err)
	}

	// Validate Go project
	root, moduleName, err := validateProject()
	if err != nil {
		return fmt.Errorf("validating project: %w", err)
	}

	fmt.Printf("→ generating registry for %s\n", cmdutil.Bold(moduleName))

	// Collect modules to fetch from hook/constructor declarations.
	mods := collectModulesToGet(out.Enabled)

	// Generate runtime registry
	wrote, err := generate.TypeRegistry(out.Enabled, dryRun)
	if err != nil {
		return fmt.Errorf("generate runtime registry: %w", err)
	}

	if !wrote {
		fmt.Printf("  %s nothing to generate — declarative templates are interpreted at runtime\n", cmdutil.Dim("○"))
		return nil
	}

	registryPath := filepath.Join(generate.TypeRegistryPackage, generate.RegistryFile)
	fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), cmdutil.Dim(registryPath))

	// Ensure main.go only when the registry was actually written.
	mainGoPath := filepath.Join("cmd", "inrun", "main.go")
	if err := ensureMainGo(root, moduleName, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "  %s main.go: %v\n", cmdutil.FailureMark(), err)
	} else {
		fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), cmdutil.Dim(mainGoPath))
	}

	if len(mods) > 0 {
		if err := goGetModules(mods, perModuleTimeout, dryRun); err != nil {
			return fmt.Errorf("fetching declared module versions: %w", err)
		}
	}

	return nil
}

// validateProject verifies the current working directory is inside a Go module.
func validateProject() (string, string, error) {
	root, err := findProjectRoot()
	if err != nil {
		return "", "", fmt.Errorf("finding project root: %w", err)
	}

	moduleName, err := readModuleName(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", "", fmt.Errorf("reading module name: %w", err)
	}

	logger.Debug().
		Str("project_root", root).
		Str("module", moduleName).
		Msg("validated Go project")

	return root, moduleName, nil
}

// ensureMainGo writes cmd/inrun/main.go with the required blank import.
func ensureMainGo(root, moduleName string, dryRun bool) error {
	targetDir := filepath.Join(root, "cmd", "inrun")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating cmd/inrun: %w", err)
	}
	mainPath := filepath.Join(targetDir, "main.go")

	timestamp := time.Now().UTC().Format(time.RFC3339)
	importLine := fmt.Sprintf(`_ "%s/pkg/typeregistry"`, moduleName)

	content := fmt.Sprintf(`// Code generated by "inrun generate registry" on %s. DO NOT EDIT.
// Re-generate by running: inrun generate registry --file <path-or-url>
package main

import (
    "context"

    "github.com/intentd/inrun/cmd/cli"
    "github.com/intentd/inrun/pkg/config"
    "github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/utils"

    %s
)

func main() {
    cfg, err := config.Init()
    if err != nil {
        logger.Fatal().AnErr("failed to load configurations", err)
        utils.Exit(err)
    }
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    cli.Execute(cfg, ctx)
}`, timestamp, importLine)

	if dryRun {
		fmt.Println(content)
		return nil
	}

	return os.WriteFile(mainPath, []byte(content), 0644)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return dir, nil
	}
	return "", fmt.Errorf("go.mod not found in current directory: %s", dir)
}

func readModuleName(modPath string) (string, error) {
	f, err := os.Open(modPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("module declaration not found in %s", modPath)
}
