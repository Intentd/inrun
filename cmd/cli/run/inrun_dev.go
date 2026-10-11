//go:build !runtime && !gateway

package run

import (
	"fmt"
	"strings"

	"github.com/inrundev/inrun/cmd/cli/cmdutil"
	"github.com/inrundev/inrun/cmd/cli/run/companions"

	"github.com/inrundev/inrun/cmd/internal"
	"github.com/inrundev/inrun/pkg/catalog"
	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/merger"
	"github.com/inrundev/inrun/pkg/registry"
	"github.com/inrundev/inrun/pkg/tools/devserver"
	"github.com/spf13/cobra"
)

// runDev is the dev build's root command. Bare `inrun` runs the Catalog in the
// current directory (or -f); `inrun <name>:<version>` pulls a published
// pattern and runs it.
func runDev(cmd *cobra.Command, args []string) error {
	dev, _ := cmd.Flags().GetBool("dev")
	devServer, _ := cmd.Flags().GetBool("dev-server")
	devServerPort, _ := cmd.Flags().GetInt("dev-server-port")
	useStack, _ := cmd.Flags().GetBool("use-stack")
	refresh, _ := cmd.Flags().GetBool("refresh")
	applyCR, _ := cmd.Flags().GetBool("apply-cr")

	// Handle dev mode cluster creation
	if err := ensureClusterReady(dev); err != nil {
		return err
	}

	// Start the mock dev server before the runtime if requested.
	if devServer {
		if err := devserver.Start(devServerPort); err != nil {
			return fmt.Errorf("starting dev server: %w", err)
		}
	}

	// If a positional OCI ref is given, pull it and resolve to a local path.
	paths, _ := cmd.Flags().GetStringSlice("file")
	if len(args) == 1 && registry.IsOCIRef(args[0]) {
		p, err := cmdutil.ResolveOCIRunPath(cmd.Context(), args[0], useStack, refresh)
		if err != nil {
			return err
		}
		paths = append([]string{p}, paths...)
	}

	// Resolve catalog paths
	paths, err := cmdutil.ResolveCatalogPaths(paths)
	if err != nil {
		return err
	}

	// Merge catalogs
	m := merger.New(paths...)
	if err := m.Merge(); err != nil {
		return fmt.Errorf("merging catalogs: %w", err)
	}

	logger.Debug().
		Strs("catalogs", paths).
		Int("total", m.Count()).
		Int("enabled", m.EnabledCount()).
		Msg("catalogs merged")

	// Apply declared crdFile, crFiles and setup paths before handing off to the runtime.
	if len(paths) > 0 {
		applyPreRuntimeResources(cmd.Context(), paths[0], m)
		if applyCR {
			applyPatternExamples(cmd.Context(), paths[0], m)
		}
	}

	// Run the runtime. Once its Catalog is built, start the console, and the
	// gateway when the Catalog serves.
	internal.RunRuntime(cmdutil.Kfg, m, cmdutil.Ctx, func(kat *catalog.Catalog) {
		companions.Start(cmdutil.Ctx, cmdutil.Kfg, paths, kat)
	})
	return nil
}

// patternArg accepts at most one argument, and only a registry reference with
// a version (name:version, registry/path:tag or oci://). A bare word is a
// command, so anything else is reported as an unknown command.
func patternArg(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if len(args) > 1 {
		return fmt.Errorf("inrun takes at most one pattern reference, got %d arguments", len(args))
	}
	if registry.IsOCIRef(args[0]) && strings.Contains(args[0], ":") {
		return nil
	}
	msg := fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
	if s := cmd.SuggestionsFor(args[0]); len(s) > 0 {
		msg += "\n\nDid you mean this?\n\t" + strings.Join(s, "\n\t")
	}
	return fmt.Errorf("%s\n\nRun '%s --help' for usage.\nTo run a pattern, give its version: inrun <name>:<version>", msg, cmd.CommandPath())
}

func init() {
	root := cmdutil.RootCmd
	root.Use = "inrun [<name>:<version>]"
	root.Args = patternArg
	root.RunE = runDev
	root.Flags().Bool("dev", false, "Create a local Kind cluster if none is reachable (development only)")
	root.Flags().Bool("dev-server", false, "Start the mock dev server for external: examples (no real services needed)")
	root.Flags().Int("dev-server-port", devserver.Port, "Port for the mock dev server")
	root.Flags().Bool("use-stack", false, "Run the pattern's stack.yaml instead of its catalog.yaml")
	root.Flags().Bool("refresh", false, "Pull the pattern again even if it is cached")
	root.Flags().Bool("apply-cr", false, "Apply the pattern's example CRD and CR before starting")
}
