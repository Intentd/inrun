//go:build !runtime && !gateway

package registry

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/serve"
	"github.com/intentd/inrun/cmd/cli/testsuite"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/pipeline"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

		patternKind, spec, files, err := registry.ValidatePatternDirectory(dir)
		if err != nil {
			return fmt.Errorf("\n  ✗ %w", err)
		}

		meta, err := registry.LoadPatternMeta(dir, spec)
		if err != nil {
			return fmt.Errorf("reading metadata: %w", err)
		}

		var providedTag string
		if refArg != "" {
			providedTag = registry.ExtractTagVersion(refArg)
		}

		if refArg == "" {
			if meta.Name == "" {
				return fmt.Errorf("metadata.name is required in %s", spec.PrimaryFile)
			}
			if meta.Version == "" {
				meta.Version = "latest"
			}
			refArg = fmt.Sprintf("%s:%s", meta.Name, meta.Version)
			providedTag = meta.Version
		} else {
			if providedTag != "" && meta.Version != "" && meta.Version != providedTag {
				msg := fmt.Errorf("%s: metadata.version %q does not match provided tag %q; use '--force' to override", spec.PrimaryFile, meta.Version, providedTag)
				if pushForce {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v (continuing due to --force)\n", cmdutil.Yellow("Warning"), msg)
					if pushUpdateMeta {
						if err := registry.PersistMetadataVersion(dir, spec.PrimaryFile, providedTag); err != nil {
							return fmt.Errorf("failed to update metadata in %s: %w", spec.PrimaryFile, err)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "updated %s: metadata.version -> %q\n", spec.PrimaryFile, providedTag)
					}
					meta.Version = providedTag
				} else {
					return msg
				}
			}
			if meta.Version == "" && providedTag != "" {
				meta.Version = providedTag
			}
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

		if patternKind == registry.BlueprintKind {
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
		}

		if patternKind == registry.BlueprintKind {
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

			if crd := registry.FindPatternFile(dir, registry.FileCRD); crd != "" && slices.Contains(files, crd) {
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
			fmt.Printf("  %s %-20s (%s)\n", cmdutil.SuccessMark(), f, cmdutil.FormatSize(info.Size()))
		}

		var simulateMeta *registry.PatternSimulate
		if patternKind == registry.BlueprintKind {
			if rel := registry.FindPatternFile(dir, registry.FileSimulate); rel != "" {
				simFile := filepath.Join(dir, rel)
				if pushForce || pushNoSimulate {
					fmt.Printf("  ~ Simulate skipped\n")
					simulateMeta = &registry.PatternSimulate{
						Status:   "skipped",
						TestedAt: time.Now().UTC().Format(time.RFC3339),
					}
				} else {
					hasAssertions := simulateFileHasAssertions(simFile)
					if !hasAssertions {
						fmt.Printf("  %s simulate.yaml has no assertions — add expect: to enforce behavior\n", cmdutil.Yellow("⚠"))
						simulateMeta = &registry.PatternSimulate{
							Status:   "no-assertion",
							TestedAt: time.Now().UTC().Format(time.RFC3339),
						}
					} else {
						fmt.Printf("\nRunning simulate gate (%s)...\n", registry.FileSimulate)
						start := time.Now()
						if err := testsuite.RunSimulateFromSpec(cmd.Context(), simFile, testsuite.CliSimulateOptions{MaxCycles: 10}); err != nil {
							return fmt.Errorf("✗ Simulate gate failed — push blocked\n  Run 'inrun simulate' to see the failures\n  Use --force to override (recorded in the artifact)\n\n%w", err)
						}
						dur := time.Since(start).Round(time.Millisecond).String()
						fmt.Printf("  %s Simulate passed (%s)\n", cmdutil.SuccessMark(), dur)
						simulateMeta = &registry.PatternSimulate{
							Status:     "passed",
							Duration:   dur,
							TestedAt:   time.Now().UTC().Format(time.RFC3339),
							Assertions: countSimulateAssertions(simFile),
						}
					}
				}
			}
		}

		var e2eMeta *registry.PatternE2E
		if patternKind == registry.BlueprintKind {
			e2eFile := pushE2EFile
			if e2eFile == "" {
				if rel := registry.FindPatternFile(dir, registry.FileE2E); rel != "" {
					e2eFile = filepath.Join(dir, rel)
				}
			} else if !filepath.IsAbs(e2eFile) {
				e2eFile = filepath.Join(dir, e2eFile)
			}
			if _, err := os.Stat(e2eFile); err == nil {
				if pushForce || pushNoE2E {
					fmt.Printf("  ~ E2E skipped\n")
					e2eMeta = &registry.PatternE2E{
						Status:   "skipped",
						TestedAt: time.Now().UTC().Format(time.RFC3339),
						Runner:   detectRunner(),
					}
				} else {
					fmt.Printf("\nRunning E2E gate (%s)...\n", registry.FileE2E)
					chart, err := cmdutil.ChartDir(pushE2EChart)
					if err != nil {
						return err
					}
					runner, err := e2e.New(e2eFile, e2e.Options{
						ClusterCtx: pushE2ECluster, UseCurrentCtx: pushE2EUseCurrent, Workers: pushE2EWorkers,
						InrunVersion: pushE2EVersion, InrunChart: chart, ValueFiles: pushE2EValues, HelmArgs: cmdutil.HelmSetArgs(pushE2ESet),
					})
					if err != nil {
						return fmt.Errorf("e2e gate: %w\n\nUse --force or --no-e2e to skip", err)
					}
					result, err := runner.Run(cmd.Context())
					if err != nil {
						return fmt.Errorf("✗ E2E gate failed — push blocked\n  Run 'inrun e2e' to see the failures\n  Use --force to override (recorded in the artifact)\n\n%w", err)
					}
					fmt.Printf("  %s E2E passed (%s)\n", cmdutil.SuccessMark(), result.Duration())
					e2eMeta = &registry.PatternE2E{
						Status:     "passed",
						Duration:   result.Duration(),
						TestedAt:   time.Now().UTC().Format(time.RFC3339),
						Runner:     detectRunner(),
						Assertions: result.Total(),
					}
				}
			}
			// --no-e2e was explicitly passed but no e2e.yaml exists; record as skipped
			// so inrun inspect shows ⊘ Skipped rather than - Not verified.
			if pushNoE2E && e2eMeta == nil {
				fmt.Printf("  ~ E2E skipped\n")
				e2eMeta = &registry.PatternE2E{
					Status:   "skipped",
					TestedAt: time.Now().UTC().Format(time.RFC3339),
					Runner:   detectRunner(),
				}
			}
		}

		var intentMeta *registry.PatternIntent
		if patternKind == registry.BlueprintKind && pushAddIntent != "" {
			intentFile := pushAddIntent
			if !filepath.IsAbs(intentFile) {
				intentFile = filepath.Join(dir, intentFile)
			}
			fmt.Printf("\nRunning intent play (%s)...\n", pushAddIntent)
			target, perr := serve.RunIntentPlay(filepath.Join(dir, registry.FileBlueprint), intentFile)
			status := "passed"
			if perr != nil {
				status = "failed"
				fmt.Printf("  %s Intent play failed: %s\n", cmdutil.WarningMark(), perr)
			} else {
				fmt.Printf("  %s Intent play passed (target: %s)\n", cmdutil.SuccessMark(), target)
			}
			intentMeta = &registry.PatternIntent{
				Status:   status,
				Target:   target,
				TestedAt: time.Now().UTC().Format(time.RFC3339),
			}
		}

		var typedMeta *registry.PatternTyped
		if patternKind == registry.BlueprintKind {
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

		spin := cmdutil.StartSpinner(fmt.Sprintf("Pushing %s...", refArg))
		progress := func(file string, size int64) {
			spin.Update(fmt.Sprintf("Uploading %s (%s)", file, cmdutil.FormatSize(size)))
		}

		opts := registry.PushOptions{
			E2E:            e2eMeta,
			Simulate:       simulateMeta,
			Intent:         intentMeta,
			Typed:          typedMeta,
			RuntimeVersion: runtimeVersion,
			Replaces:       replaces,
		}
		digest, err := client.Push(cmd.Context(), ref, dir, opts, progress)
		if err != nil {
			spin.Failure()
			return fmt.Errorf("push failed: %w", err)
		}
		spin.Stop()

		fmt.Printf("\n%s Pushed: %s\n", cmdutil.SuccessMark(), ref.String())
		fmt.Printf("  Digest: %s\n", digest)

		if patternKind == registry.BlueprintKind {
			moduleYAML := filepath.Join(dir, registry.FileModule)
			if _, err := os.Stat(moduleYAML); err == nil {
				moduleRef, err := registry.ResolveForKind(fmt.Sprintf("%s:%s", meta.Name, meta.Version), registry.ModuleKind)
				var moduleReplaces string
				if err == nil {
					if moduleReplaces, err = checkPublished(cmd.Context(), moduleRef); err != nil {
						fmt.Fprintf(os.Stderr, "warning: not pushing %s: %v\n", registry.FileModule, err)
					}
				}
				if err == nil {
					fmt.Printf("\nAlso pushing %s to %s...\n", registry.FileModule, moduleRef.Registry)
					spinModule := cmdutil.StartSpinner(fmt.Sprintf("Pushing %s...", registry.FileModule))
					if mDigest, err := client.Push(cmd.Context(), moduleRef, dir, registry.PushOptions{RuntimeVersion: version.Short(), Replaces: moduleReplaces}, nil); err != nil {
						spinModule.Failure()
						fmt.Fprintf(os.Stderr, "warning: module push failed: %v\n", err)
					} else {
						spinModule.Stop()
						fmt.Printf("%s Pushed module: %s\n", cmdutil.SuccessMark(), moduleRef.String())
						fmt.Printf("  Digest: %s\n", mDigest[:19]+"...")
					}
				}
			}
		}

		fmt.Printf("\nTo import in a Blueprint:\n")
		if patternKind == registry.ModuleKind {
			fmt.Printf("  imports:\n")
			fmt.Printf("    - module: %s\n", ref.String())
		} else {
			fmt.Printf("  imports:\n")
			fmt.Printf("    registry:\n")
			fmt.Printf("      - url: %s\n", ref.String())
		}

		_ = meta
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

// detectTypedBlueprint parses a blueprint.yaml and returns a PatternTyped if any
// CRD declares customHooks or customConstructor. Returns nil on parse error.
// extractRuntimeVersionFromGoMod scans go.mod for the inrun runtime dependency
// and returns its version (e.g. "v0.7.6"). Returns "" if go.mod is absent or the
// dependency is not declared.
func extractRuntimeVersionFromGoMod(dir string) string {
	f, err := os.Open(filepath.Join(dir, registry.FileGoMod))
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// inline form:  require github.com/intentd/inrun vX.Y.Z
		if strings.HasPrefix(line, "require github.com/intentd/inrun ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[2]
			}
		}
		// block form (inside require (...)):  github.com/intentd/inrun vX.Y.Z
		if strings.HasPrefix(line, "github.com/intentd/inrun ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ""
	}
	return ""
}

func detectTypedBlueprint(blueprintPath string) *registry.PatternTyped {
	bp, err := blueprint.ParseFile(blueprintPath)
	if err != nil {
		return nil
	}
	var t registry.PatternTyped
	for _, name := range bp.CRDNames() {
		entry, ok := bp.CRDEntry(name)
		if !ok {
			continue
		}
		if entry.CustomHooksEnabled() {
			t.HasHooks = true
		}
		if entry.ConstructorEnabled() {
			t.HasConstructor = true
		}
	}
	if !t.HasHooks && !t.HasConstructor {
		return nil
	}
	return &t
}

// simulateFileHasAssertions returns true when simulate.yaml contains an
// expect: block — meaning the simulate gate will actually assert something.
func simulateFileHasAssertions(path string) bool {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return false
	}
	var doc struct {
		Spec *struct {
			Expect *struct{} `yaml:"expect"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	return doc.Spec != nil && doc.Spec.Expect != nil
}

// countSimulateAssertions counts the total number of discrete assertions in
// simulate.yaml: each ops/absent rule counts as one, plus one each for
// steady, steadyAt, and noErrors when set. Recurses into crds: sub-expects.
func countSimulateAssertions(path string) int {
	data, err := cmdutil.ReadLocal(path)
	if err != nil {
		return 0
	}
	type expectBlock struct {
		Steady   *bool                   `yaml:"steady"`
		SteadyAt *int                    `yaml:"steadyAt"`
		NoErrors bool                    `yaml:"noErrors"`
		Ops      []struct{}              `yaml:"ops"`
		Absent   []struct{}              `yaml:"absent"`
		CRDs     map[string]*expectBlock `yaml:"crds"`
	}
	var doc struct {
		Spec *struct {
			Expect *expectBlock `yaml:"expect"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Spec == nil || doc.Spec.Expect == nil {
		return 0
	}
	var count func(e *expectBlock) int
	count = func(e *expectBlock) int {
		if e == nil {
			return 0
		}
		n := len(e.Ops) + len(e.Absent)
		if e.Steady != nil {
			n++
		}
		if e.SteadyAt != nil {
			n++
		}
		if e.NoErrors {
			n++
		}
		for _, crd := range e.CRDs {
			n += count(crd)
		}
		return n
	}
	return count(doc.Spec.Expect)
}

func detectRunner() string {
	switch {
	case os.Getenv("GITHUB_ACTIONS") == "true":
		return "github-actions"
	case os.Getenv("GITLAB_CI") == "true":
		return "gitlab-ci"
	case os.Getenv("CIRCLECI") == "true":
		return "circleci"
	case os.Getenv("JENKINS_URL") != "":
		return "jenkins"
	case os.Getenv("BUILDKITE") == "true":
		return "buildkite"
	case os.Getenv("DRONE") == "true":
		return "drone"
	case os.Getenv("CI") == "true":
		return "ci"
	default:
		return "local"
	}
}

// checkPublished returns the digest ref already has in the registry, for the
// new artifact to record, or "" when ref is not published. Without
// --overwrite, a published ref is an error.
func checkPublished(ctx context.Context, ref *registry.Ref) (string, error) {
	client, err := registry.NewClient()
	if err != nil {
		return "", err
	}
	current, err := client.Published(ctx, ref)
	if err != nil {
		return "", err
	}
	if current != "" && !pushOverwrite {
		return "", fmt.Errorf("%s is already published\n  Bump metadata.version to publish a change, or pass --overwrite to replace it", ref.String())
	}
	return current, nil
}
