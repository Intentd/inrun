//go:build !runtime && !gateway

package testsuite

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/validate"

	"github.com/intentd/inrun/pkg/registry/simulate"
)

// listSimulateDiscovery lists the files that would be discovered without running them.
func listSimulateDiscovery(root string, skip []string, limit int) error {
	_, paths, err := discoverSimulate(root, skip)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		fmt.Printf("No simulate files found under %s\n", root)
		return nil
	}

	fmt.Printf("→ Would run %d simulate file(s) under %s\n\n", len(paths), root)

	display := len(paths)
	if limit > 0 && limit < display {
		display = limit
	}

	for _, p := range paths[:display] {
		fmt.Printf("  %s\n", p)
	}

	if display < len(paths) {
		fmt.Printf("  ... %d more\n", len(paths)-display)
	}

	return nil
}

// runSimulateDiscovery finds all simulate.yaml files under root, simulates each,
// and prints an aggregate summary. It returns the files' results folded into
// one suite result, each file an import of it — the shape an imports:-only
// simulate.yaml gives — for the report; nil when there is nothing to run.
func runSimulateDiscovery(ctx context.Context, root string, skip []string, simOpts CliSimulateOptions) (*simulate.Result, error) {
	paths, rel, err := discoverSimulate(root, skip)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		fmt.Printf("No simulate files found under %s\n", root)
		return nil, nil
	}

	suite := &simulate.Result{Name: root}
	suiteStart := time.Now()
	finish := func(err error) (*simulate.Result, error) {
		suite.Elapsed = time.Since(suiteStart)
		suite.Assertions.Status = suite.SimulateStatusFor()
		return suite, err
	}

	fmt.Printf("Simulating %d file(s) under %s\n\n", len(paths), root)

	// In discovery mode each file declares its own target; don't let a CLI
	// --target flag override every file in the suite.
	fileOpts := simOpts
	fileOpts.Target = ""

	var results []simulateFileResult
	for i, p := range paths {
		start := time.Now()
		fileRes, err := RunSimulateFromSpec(ctx, p, fileOpts)
		elapsed := time.Since(start)
		suite.AddImport(rel[i], fileRes, err)

		res := simulateFileResult{
			path:    rel[i],
			elapsed: elapsed,
		}

		if err != nil {
			fmt.Printf("  %-55s %s  %s\n",
				rel[i],
				cmdutil.Red("✗ "+err.Error()),
				cmdutil.Dim("[assert]"),
			)
			res.cycleErrs = true
		} else {
			fmt.Printf("  %-55s %s  %s\n",
				rel[i],
				cmdutil.Green(fmt.Sprintf("✓ passed (%s)", elapsed.Round(time.Millisecond))),
				cmdutil.Dim("[assert]"),
			)
			res.steady = true
		}

		results = append(results, res)
	}

	// Aggregate summary.
	var simulated, skipped int
	var slowest simulateFileResult

	for _, r := range results {
		if r.skipped {
			skipped++
			continue
		}

		simulated++
		if r.elapsed > slowest.elapsed {
			slowest = r
		}
	}

	fmt.Printf("\n  %d file(s) — %d simulated, %d skipped\n",
		len(results), simulated, skipped)

	if slowest.path != "" {
		fmt.Printf("  Slowest: %s (%s)\n",
			slowest.path, slowest.elapsed.Round(time.Millisecond))
	}

	var errFiles []simulateFileResult
	for _, r := range results {
		if r.cycleErrs {
			errFiles = append(errFiles, r)
		}
	}

	if len(errFiles) > 0 {
		fmt.Printf("\n  %s — run directly for full output:\n",
			cmdutil.Yellow("Files with errors"))

		for _, r := range errFiles {
			fmt.Printf("    inrun simulate -f %s\n", r.path)
		}

		return finish(fmt.Errorf("simulation failed in %d file(s)", len(errFiles)))
	}

	return finish(nil)
}

// validateSimulateDiscovery validates every simulate file discovered under root
// and prints one line per file: ok or the first error.
func validateSimulateDiscovery(root string, skip []string) error {
	abs, rel, err := discoverSimulate(root, skip)
	if err != nil {
		return err
	}
	if len(abs) == 0 {
		fmt.Printf("No simulate files found under %s\n", root)
		return nil
	}

	fmt.Printf("→ Validating %d simulate file(s) under %s\n\n", len(abs), root)

	var failed int
	for i, p := range abs {
		if err := validate.ValidateSimulateFileQuiet(p); err != nil {
			failed++
			fmt.Printf("  %s %s\n", cmdutil.FailureMark(), rel[i])

			for _, line := range strings.Split(strings.TrimSpace(err.Error()), "\n") {
				fmt.Printf("      %s\n", line)
			}
			continue
		}

		fmt.Printf("  %s %s\n", cmdutil.SuccessMark(), rel[i])
	}

	fmt.Println()

	if failed > 0 {
		return fmt.Errorf("%d of %d file(s) failed validation", failed, len(abs))
	}

	fmt.Printf("%d file(s) valid\n", len(abs))
	return nil
}

// discoverSimulate returns discovered simulate files as absolute paths and
// paths relative to root.
func discoverSimulate(root string, skip []string) (abs, rel []string, err error) {
	abs, err = simulate.DiscoverSimulateFiles(root, structured.SplitCommaList(skip))
	if err != nil {
		return nil, nil, fmt.Errorf("discovery: %w", err)
	}

	if len(abs) == 0 {
		return nil, nil, nil
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving root %s: %w", root, err)
	}

	rel = make([]string, len(abs))
	for i, p := range abs {
		rel[i], err = filepath.Rel(absRoot, p)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving relative path for %s: %w", p, err)
		}
	}

	return abs, rel, nil
}
