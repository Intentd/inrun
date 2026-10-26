//go:build !runtime && !gateway

package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/tools/generate"
	"github.com/spf13/cobra"
)

var createPatternCmd = &cobra.Command{
	Use:   "pattern",
	Short: "Scaffold a new Inrun pattern: blueprint.yaml, simulate.yaml, e2e.yaml, README.md",
	Long: `Creates the files needed to build, test, and publish an Inrun pattern.

Always written:
  blueprint.yaml   — operator declaration
  simulate.yaml  — in-memory test scaffold (inrun simulate)
  e2e.yaml       — real-cluster integration test scaffold (inrun e2e)
  README.md      — actionable steps from edit to release

Also written when --typed, --add-hook, or --add-constructor:
  values.yaml    — runtime image (set before inrun e2e)
  Makefile       — registry, build, build-runtime, docker, push, release
  Dockerfile     — production container image (distroless, runtime binary only)

Typed mode flags are forwarded to blueprint generation:
  --add-hook          Include a hooks section
  --add-constructor   Include a constructor section
  --typed             Include both hooks and constructor (commented)

Examples:
  inrun create pattern
  inrun create pattern --add-hook -o ./my-operator/
  inrun create pattern --typed`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addHook, _ := cmd.Flags().GetBool("add-hook")
		addConstructor, _ := cmd.Flags().GetBool("add-constructor")
		typed, _ := cmd.Flags().GetBool("typed")
		outputDir, _ := cmd.Flags().GetString("output")

		if outputDir == "" {
			outputDir = "."
		}

		isTyped := typed || addHook || addConstructor

		blueprintOpts := generate.BlueprintScaffoldOptions{
			AddHook:        addHook,
			AddConstructor: addConstructor,
			Typed:          typed,
			OutputFile:     filepath.Join(outputDir, cmdutil.FileBlueprint),
		}
		if err := blueprintOpts.Validate(); err != nil {
			return err
		}

		fmt.Printf("generating pattern scaffold → %s/\n", outputDir)

		if _, err := generate.BlueprintScaffold(blueprintOpts); err != nil {
			return fmt.Errorf("generating %s: %w", cmdutil.FileBlueprint, err)
		}

		if err := generate.WriteSimulateScaffold(filepath.Join(outputDir, cmdutil.DirTest, cmdutil.FileSimulate)); err != nil {
			return fmt.Errorf("generating %s: %w", cmdutil.FileSimulate, err)
		}

		if err := generate.WriteE2EScaffold(filepath.Join(outputDir, cmdutil.DirTest, cmdutil.FileE2e), isTyped); err != nil {
			return fmt.Errorf("generating %s: %w", cmdutil.FileE2e, err)
		}

		if err := generate.WriteREADME(filepath.Join(outputDir, cmdutil.FileReadMe), isTyped); err != nil {
			return fmt.Errorf("generating %s: %w", cmdutil.FileReadMe, err)
		}

		if isTyped {
			if err := generate.WriteValuesYAML(filepath.Join(outputDir, cmdutil.DirTest, cmdutil.FileValues)); err != nil {
				return fmt.Errorf("generating %s: %w", cmdutil.FileValues, err)
			}
			if err := generate.WriteMakefile(filepath.Join(outputDir, cmdutil.FileMakeFile)); err != nil {
				return fmt.Errorf("generating %s: %w", cmdutil.FileMakeFile, err)
			}
			if err := generate.WriteDockerfile(filepath.Join(outputDir, cmdutil.FileDockerfile)); err != nil {
				return fmt.Errorf("generating %s: %w", cmdutil.FileDockerfile, err)
			}
		}

		fmt.Printf("\n→ pattern scaffold written to %s\n", cmdutil.Bold(outputDir+"/"))
		fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.FileBlueprint, cmdutil.Dim("declare your CRD(s) and resources"))
		fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.DirTest+"/"+cmdutil.FileSimulate, cmdutil.Dim("inrun simulate"))
		fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.DirTest+"/"+cmdutil.FileE2e, cmdutil.Dim("inrun e2e"))
		fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.FileReadMe, cmdutil.Dim("start here"))
		if isTyped {
			fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.DirTest+"/"+cmdutil.FileValues, cmdutil.Dim("set runtime.image before inrun e2e"))
			fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.FileMakeFile, cmdutil.Dim("make registry, make build, make release"))
			fmt.Printf("  %s %-16s %s\n", cmdutil.SuccessMark(), cmdutil.FileDockerfile, cmdutil.Dim("production container image"))
		}
		fmt.Println()
		return nil
	},
}

func init() {
	cmdutil.CreateCmd.AddCommand(createPatternCmd)
	createPatternCmd.Flags().Bool("add-hook", false,
		"Typed mode: include a hooks section in blueprint.yaml (also writes Makefile + Dockerfile)")
	createPatternCmd.Flags().Bool("add-constructor", false,
		"Typed mode: include a constructor section in blueprint.yaml (also writes Makefile + Dockerfile)")
	createPatternCmd.Flags().Bool("typed", false,
		"Typed mode: include both hooks and constructor commented (also writes Makefile + Dockerfile)")
	createPatternCmd.Flags().StringP("output", "o", "",
		"Output directory (default: current directory)")
}
