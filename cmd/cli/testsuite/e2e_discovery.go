//go:build !runtime && !gateway

package testsuite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/validate"

	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/util/structured"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

// listE2EDiscovery lists the files that would be discovered without running them.
func listE2EDiscovery(root string, skip []string, limit int) error {
	_, paths, err := discoverE2E(root, skip)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}

	fmt.Printf("→ Would run %d e2e file(s) under %s\n\n", len(paths), root)

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

// runE2EDiscovery finds all *e2e.yaml leaf files under root, builds a temp
// aggregator, and runs it as a normal suite.
func runE2EDiscovery(
	cmd *cobra.Command,
	root, wait string,
	skip []string,
	opts e2e.Options,
) error {
	paths, rel, err := discoverE2E(root, skip)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}

	chart, err := chartFlag(cmd)
	if err != nil {
		return err
	}
	opts.InrunChart = chart

	fmt.Printf("→ Discovered %d e2e file(s) under %s\n", len(paths), root)
	for _, p := range rel {
		fmt.Printf("    %s\n", p)
	}
	fmt.Println()

	suite := e2e.BuildDiscoveryE2E(paths, wait)

	// Write to a temp file so the runner resolves relative paths correctly.
	tmp, err := os.CreateTemp("", "inrun-e2e-discovery-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp suite file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if err := yaml.NewEncoder(tmp).Encode(suite); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp suite: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp suite: %w", err)
	}

	runner, err := e2e.New(tmp.Name(), opts)
	if err != nil {
		return err
	}
	_, err = runner.Run(cmd.Context())
	return err
}

// validateE2EDiscovery validates every e2e file discovered under root and
// prints one line per file: ok or the first error. Returns an error when any
// file fails validation.
func validateE2EDiscovery(root string, skip []string) error {
	abs, rel, err := discoverE2E(root, skip)
	if err != nil {
		return err
	}
	if len(abs) == 0 {
		return nil
	}

	fmt.Printf("→ Validating %d e2e file(s) under %s\n\n", len(abs), root)

	var failed int
	for i, p := range abs {
		if err := validate.ValidateE2EFileQuiet(p); err != nil {
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

// discoverE2E returns the discovered E2E files as absolute paths and paths
// relative to root. It prints a message when no files are found.
func discoverE2E(root string, skip []string) (abs, rel []string, err error) {
	abs, err = e2e.DiscoverE2EFiles(root, structured.SplitCommaList(skip))
	if err != nil {
		return nil, nil, fmt.Errorf("discovery: %w", err)
	}

	if len(abs) == 0 {
		fmt.Printf("No e2e files found under %s\n", root)
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
