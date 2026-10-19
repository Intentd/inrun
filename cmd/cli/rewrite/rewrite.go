//go:build !runtime && !gateway

// package rewrite implements the rewrite command, which converts a
// controller-runtime reconciler to run on this runtime.
package rewrite

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/tools/rewrite"
	"github.com/intentd/inrun/pkg/version"
	"github.com/spf13/cobra"
)

var rewriteCmd = &cobra.Command{
	Use:   "rewrite <file>",
	Short: "Rewrite a controller-runtime operator to Inrun",
	Long: `Rewrites a controller-runtime reconciler to Inrun. Your Reconcile logic
is untouched — Inrun takes over the infrastructure.

Default mode (--mode toclient): zero changes to your Reconcile signature or
call sites. SetupWithManager is removed; a two-line constructor using
adapter.ToClient and domain.ReconcilerFrom is injected. Your reconciler
compiles and runs inside Inrun with no other edits.

  inrun rewrite ./controller/webapp_controller.go -o ./my-operator

The output directory receives the rewritten file plus scaffolding:
blueprint.yaml, simulate.yaml, e2e.yaml, go.mod, Makefile, Dockerfile.

For a full rewrite to Inrun style, use --mode native.

Examples:
  inrun rewrite ./controller/webapp_controller.go -o ./my-operator
  inrun rewrite ./controller/webapp_controller.go --mode native -o ./out
  inrun rewrite ./controller/webapp_controller.go  # prompts before replacing`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath := args[0]
		outputDir, _ := cmd.Flags().GetString("output")
		modulePath, _ := cmd.Flags().GetString("module")
		operatorName, _ := cmd.Flags().GetString("name")
		modeFlag, _ := cmd.Flags().GetString("mode")

		var mode rewrite.Mode
		switch modeFlag {
		case "native":
			mode = rewrite.ModeNative
		case "toclient", "":
			mode = rewrite.ModeToClient
		default:
			return fmt.Errorf("unknown --mode %q: valid values are toclient, native", modeFlag)
		}

		src, err := cmdutil.ReadLocal(inputPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", inputPath, err)
		}

		res, err := rewrite.Apply(src, mode)
		if err != nil {
			return fmt.Errorf("rewrite: %w", err)
		}

		if len(res.Warnings) > 0 {
			fmt.Fprintf(os.Stderr, "\n%s\n", cmdutil.Yellow("Warnings — review these before running:"))
			for _, w := range res.Warnings {
				fmt.Fprintf(os.Stderr, "  %s %s\n", cmdutil.Yellow("⚠"), w)
			}
			fmt.Fprintln(os.Stderr)
		}

		opts := rewrite.Options{
			ModulePath:   modulePath,
			OperatorName: operatorName,
			InrunVersion: version.Short(),
		}
		files := rewrite.Generate(res, opts)

		if outputDir != "" {
			return writeOutputDir(outputDir, inputPath, res, files)
		}

		return replaceInPlace(inputPath, res, files)
	},
}

func writeOutputDir(dir, inputPath string, res *rewrite.Result, files rewrite.Files) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	reconcilerDest := filepath.Join(dir, filepath.Base(inputPath))
	if err := os.WriteFile(reconcilerDest, res.Source, 0o644); err != nil {
		return fmt.Errorf("write reconciler: %w", err)
	}
	fmt.Printf("  %s %s\n", cmdutil.Green("→"), reconcilerDest)

	type namedFile struct {
		name    string
		content string
	}
	generated := []namedFile{
		{"blueprint.yaml", files.Blueprint},
		{"test/simulate.yaml", files.Simulate},
		{"test/e2e.yaml", files.E2E},
		{"go.mod", files.GoMod},
		{"Makefile", files.Makefile},
		{"Dockerfile", files.Dockerfile},
		{"README.md", files.README},
	}
	for _, f := range generated {
		dest := filepath.Join(dir, f.name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
		fmt.Printf("  %s %s\n", cmdutil.Green("→"), dest)
	}

	fmt.Printf("\n%s Search for %s in %s to see what needs attention.\n",
		cmdutil.Green("✓ Done."), cmdutil.Bold("TODO(inrun rewrite)"), dir)
	return nil
}

func replaceInPlace(inputPath string, res *rewrite.Result, files rewrite.Files) error {
	dir := filepath.Dir(inputPath)

	fmt.Printf("This will replace %s and write %s alongside it.\n",
		cmdutil.Bold(inputPath), cmdutil.Bold("blueprint.yaml, test/simulate.yaml, test/e2e.yaml, go.mod, Makefile, Dockerfile"))
	fmt.Printf("%s [y/N] ", cmdutil.Yellow("Continue?"))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	if err := os.WriteFile(inputPath, res.Source, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", inputPath, err)
	}
	fmt.Printf("  %s %s\n", cmdutil.Green("→"), inputPath)

	type namedFile struct {
		name    string
		content string
	}
	generated := []namedFile{
		{"blueprint.yaml", files.Blueprint},
		{"test/simulate.yaml", files.Simulate},
		{"test/e2e.yaml", files.E2E},
		{"go.mod", files.GoMod},
		{"Makefile", files.Makefile},
		{"Dockerfile", files.Dockerfile},
	}
	for _, f := range generated {
		dest := filepath.Join(dir, f.name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
		}
		if err := os.WriteFile(dest, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.name, err)
		}
		fmt.Printf("  %s %s\n", cmdutil.Green("→"), dest)
	}

	fmt.Printf("\n%s Search for %s to see what needs attention.\n",
		cmdutil.Green("✓ Done."), cmdutil.Bold("TODO(inrun rewrite)"))
	return nil
}

func init() {
	cmdutil.RootCmd.AddCommand(rewriteCmd)

	rewriteCmd.Flags().StringP("output", "o", "", "Write output to this directory (non-destructive; skips confirmation)")
	rewriteCmd.Flags().String("module", "", "Go module path for the rewritten operator (e.g. github.com/myorg/my-operator)")
	rewriteCmd.Flags().String("name", "", "Operator name in kebab-case (e.g. my-operator); derived from receiver type if omitted")
	rewriteCmd.Flags().String("mode", "toclient", "Migration mode: toclient (default, zero Reconcile changes) or native (full rewrite)")

	// Shadow global flags so they don't appear under `inrun rewrite`
	cmdutil.ShadowGlobalCommandFlags(rewriteCmd, "file")
}
