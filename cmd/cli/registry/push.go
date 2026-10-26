//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/util/version"
	"github.com/spf13/cobra"
)

// ── push ──────────────────────────────────────────────────────────────────────

var (
	pushForce         bool
	pushUpdateMeta    bool
	pushE2EFile       string
	pushNoE2E         bool
	pushNoSimulate    bool
	pushE2ECluster    string
	pushE2EUseCurrent bool
	pushE2EWorkers    int
	pushE2EChart      string
	pushE2EVersion    string
	pushE2EValues     []string
	pushE2ESet        []string
	pushOverwrite     bool
	pushAddIntent     string
)

var pushCmd = &cobra.Command{
	Use:   "push <name>:<version> <dir>  OR  push <dir>",
	Short: "Push a pattern or module directory to the registry",
	Args:  cobra.RangeArgs(1, 2),
	Example: `  inrun push postgres:v14 ./patterns/postgres/
  inrun push redis:v7 ./modules/redis/
  INRUN_REGISTRY=oci://myregistry.io/patterns inrun push payments:v1.0 ./payments/
  inrun push .   # use metadata.name:metadata.version from the pattern`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			refArg string
			dirArg string
		)

		if len(args) == 2 {
			refArg = args[0]
			dirArg = args[1]
		} else {
			refArg = ""
			dirArg = args[0]
		}

		dir, err := filepath.Abs(dirArg)
		if err != nil {
			return err
		}

		patternKind, spec, files, err := registry.ValidatePatternDirectory(dir, false)
		if err != nil {
			return fmt.Errorf("\n  ✗ %w", err)
		}

		meta, err := registry.LoadPatternMeta(dir, spec)
		if err != nil {
			return fmt.Errorf("reading metadata: %w", err)
		}

		refArg, meta.Version, err = resolvePushVersion(
			cmd, dir, spec.PrimaryFile, meta.Name, meta.Version, refArg,
			pushForce, pushUpdateMeta,
		)
		if err != nil {
			return err
		}

		ref, err := registry.ResolveForKind(refArg, patternKind)
		if err != nil {
			return fmt.Errorf("invalid reference: %w", err)
		}

		// A published version is never replaced by accident: consumers who pulled
		// it, and the proof recorded for it, refer to what was there. Checked
		// before the gates, so a refused push doesn't wait for them first. With
		// --overwrite, the new artifact records the digest it replaces.
		replaces, err := checkPublished(cmd.Context(), ref)
		if err != nil {
			return err
		}

		cmdutil.PrintBanner()
		fmt.Printf("Pushing %s (%s) to %s...\n", refArg, patternKind, ref.Registry)

		if err := preparePushFiles(dir, patternKind.IsBlueprint(), files); err != nil {
			return err
		}

		simulateMeta, e2eMeta, intentMeta, err := runPushGates(
			cmd, dir, patternKind.SupportsGates(),
		)
		if err != nil {
			return err
		}

		var typedMeta *registry.PatternTyped
		if patternKind.IsBlueprint() {
			typedMeta = detectTypedBlueprint(filepath.Join(dir, registry.FileBlueprint))
		}

		runtimeVersion := version.Short()
		if typedMeta != nil {
			if v := extractRuntimeVersionFromGoMod(dir); v != "" {
				runtimeVersion = v
			}
		}

		client, err := registry.NewClient()
		if err != nil {
			return fmt.Errorf("initializing client: %w", err)
		}

		opts := registry.PushOptions{
			E2E:            e2eMeta,
			Simulate:       simulateMeta,
			Intent:         intentMeta,
			Typed:          typedMeta,
			RuntimeVersion: runtimeVersion,
			Replaces:       replaces,
		}

		digest, err := pushWithProgress(
			fmt.Sprintf("Pushing %s...", refArg),
			func(progress func(string, int64)) (string, error) {
				return client.Push(cmd.Context(), ref, dir, opts, progress)
			},
		)
		if err != nil {
			return fmt.Errorf("push failed: %w", err)
		}

		fmt.Printf("\n%s Pushed: %s\n", cmdutil.SuccessMark(), ref.String())
		fmt.Printf("  Digest: %s\n", digest)

		if patternKind.IsBlueprint() {
			publishOptionalModule(dir, func() (string, string, string, error) {
				moduleRef, err := registry.ResolveForKind(
					fmt.Sprintf("%s:%s", meta.Name, meta.Version),
					registry.ModuleKind,
				)
				if err != nil {
					return "", "", "", err
				}

				moduleReplaces, err := checkPublished(cmd.Context(), moduleRef)
				if err != nil {
					return moduleRef.String(), "", "not pushing " + registry.FileModule, err
				}

				fmt.Printf("\nAlso pushing %s to %s...\n", registry.FileModule, moduleRef.Registry)

				mDigest, err := pushWithProgress(
					fmt.Sprintf("Pushing %s...", registry.FileModule),
					func(progress func(string, int64)) (string, error) {
						return client.Push(
							cmd.Context(),
							moduleRef,
							dir,
							registry.PushOptions{
								RuntimeVersion: version.Short(),
								Replaces:       moduleReplaces,
							},
							nil,
						)
					},
				)
				if err != nil {
					return moduleRef.String(), "", "module push failed", err
				}

				return moduleRef.String(), mDigest, "", nil
			})
		}

		printImportInstructions(ref.String(), patternKind.IsModule())

		return nil
	},
}

func init() {
	pushCmd.Flags().BoolVar(&pushForce, "force", false, "Force push even if metadata.version differs from tag or e2e fails")
	pushCmd.Flags().BoolVar(&pushUpdateMeta, "update-meta", false, "Persist overridden metadata.version back to the primary file")
	pushCmd.Flags().StringVar(&pushE2EFile, "e2e", "", "Path to e2e spec file (default: e2e.yaml in pattern dir)")
	pushCmd.Flags().BoolVar(&pushNoE2E, "no-e2e", false, "Skip the e2e gate even if e2e.yaml is present")
	pushCmd.Flags().BoolVar(&pushNoSimulate, "no-simulate", false, "Skip the simulate gate even if simulate.yaml is present")
	pushCmd.Flags().StringVar(&pushE2ECluster, "cluster", "", "Reuse an existing kind cluster context for the e2e gate (skips cluster creation)")
	pushCmd.Flags().BoolVar(&pushE2EUseCurrent, "use-current", false, "Use the current kubeconfig context for the e2e gate (skips cluster creation)")
	pushCmd.Flags().IntVar(&pushE2EWorkers, "workers", 0, "Number of kind worker nodes for the e2e gate cluster (0 = control-plane only)")
	pushCmd.Flags().BoolVar(&pushOverwrite, "overwrite", false, "Replace a version that is already published (anyone who pulled it keeps the old content)")
	pushCmd.Flags().StringVar(&pushE2EChart, "chart", "", "Local Inrun chart directory for the e2e gate to install instead of the published chart")
	pushCmd.Flags().StringVar(&pushE2EVersion, "version", "", "Inrun version for the e2e gate to install (e.g. v1.2.3)")
	pushCmd.Flags().StringSliceVar(&pushE2EValues, "values", nil, "Helm values files for the e2e gate's Inrun install")
	pushCmd.Flags().StringSliceVar(&pushE2ESet, "set", nil, "Helm --set arguments for the e2e gate's Inrun install (e.g. key=value)")
	pushCmd.Flags().StringVar(&pushAddIntent, "add-intent", "", "Run inrun serve play against this intent file (YAML or JSON) and bake the result into the artifact")
	cmdutil.RootCmd.AddCommand(pushCmd)

	// Shadow global flags so they don't appear under `inrun push`
	cmdutil.ShadowGlobalCommandFlags(pushCmd, "file")
}
