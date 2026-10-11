//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
	"github.com/spf13/cobra"
)

// ── inspect ───────────────────────────────────────────────────────────────────

var inspectModule bool

var inspectCmd = &cobra.Command{
	Use:   "inspect <name>:<version>",
	Short: "Show metadata for a pattern version",
	Args:  cobra.ExactArgs(1),
	Example: `  inrun inspect postgres:v14
  inrun inspect web-service:v1.0.0 --module
  inrun inspect oci://ghcr.io/myorg/patterns/redis:v7
  inrun inspect redis:v1.0.0 --view catalog.yaml,simulate.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		kind := registry.CatalogKind
		if inspectModule {
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

		if versionsFlag, _ := cmd.Flags().GetBool("versions"); versionsFlag {
			spin := cmdutil.StartSpinner("Fetching version history...")
			versions, err := client.ListVersions(cmd.Context(), ref, 10)
			if err != nil {
				spin.Failure()
				return fmt.Errorf("listing versions: %w", err)
			}
			spin.Stop()

			name := ref.ShortName()
			if idx := strings.LastIndex(name, ":"); idx != -1 {
				name = name[:idx]
			}
			versionWord := "versions"
			if len(versions) == 1 {
				versionWord = "version"
			}
			fmt.Printf("\n%s  (%d %s)\n\n", cmdutil.Bold(name), len(versions), versionWord)
			const (
				tagW = 12
				simW = 27
			)
			if !inspectModule {
				fmt.Printf("  %s  %s  %s\n",
					cmdutil.Gray(fmt.Sprintf("%-*s", tagW, "VERSION")),
					cmdutil.PadRight(cmdutil.Gray("SIMULATE"), simW),
					cmdutil.Gray("E2E"))
			}
			for i, v := range versions {
				latest := ""
				deprecated := ""
				if i == 0 {
					latest = "  ← latest"
				}
				if v.Meta.Deprecated != nil {
					dep := v.Meta.Deprecated
					d := &types.CatalogDeprecation{
						MigratedTo: dep.MigratedTo,
						Message:    dep.Message,
					}
					if dep.TimelineFrom != "" || dep.TimelineTo != "" {
						d.Timeline = &types.DeprecationTimeline{
							From: dep.TimelineFrom,
							To:   dep.TimelineTo,
						}
					}
					state := d.DeprecationState(time.Now())
					switch state {
					case "eol":
						deprecated = cmdutil.Red(" ✗ EOL")
					case "warning":
						deprecated = cmdutil.Yellow(" ⚠ deprecated")
					}
				}
				if inspectModule {
					fmt.Printf("  %-*s%s\n", tagW, v.Tag, latest)
					continue
				}
				var simCol, e2eCol string
				if v.Meta.Simulate != nil {
					switch v.Meta.Simulate.Status {
					case "passed":
						suffix := ""
						if v.Meta.Simulate.Assertions > 0 {
							suffix = fmt.Sprintf("%d assertions", v.Meta.Simulate.Assertions)
						}
						simCol = cmdutil.SimulateVerified(suffix)
					case "skipped":
						simCol = cmdutil.SkippedShort()
					case "no-assertion":
						simCol = cmdutil.NoAssertion()
					}
				} else {
					simCol = cmdutil.Gray("- Not verified")
				}
				if v.Meta.E2E != nil {
					switch v.Meta.E2E.Status {
					case "passed":
						suffix := ""
						if v.Meta.E2E.Duration != "" {
							suffix = v.Meta.E2E.Duration
						}
						e2eCol = cmdutil.E2eVerified(suffix)
					case "skipped":
						e2eCol = cmdutil.SkippedShort()
					}
				} else {
					e2eCol = cmdutil.E2eNotVerified()
				}
				fmt.Printf("  %-*s  %s  %s%s%s\n", tagW, v.Tag, cmdutil.PadRight(simCol, simW), e2eCol, latest, deprecated)
			}
			fmt.Println()
			return nil
		}

		info, err := client.Info(cmd.Context(), ref)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") {
				hint := fmt.Sprintf("\n\nhint: authenticate first:\n  docker login %s", ref.Registry)
				if !inspectModule {
					hint += "\nhint: if this is a module, re-run with --module"
				}
				return fmt.Errorf("fetching info: %w%s", err, hint)
			}
			return fmt.Errorf("fetching info: %w", err)
		}
		m := info.Meta

		// --view: skip the metadata block, just fetch and print requested files.
		viewArg, _ := cmd.Flags().GetString("view")
		if viewArg != "" {
			fileMap := make(map[string]registry.FileEntry, len(info.Files))
			available := make([]string, 0, len(info.Files))
			for _, f := range info.Files {
				fileMap[f.Name] = f
				available = append(available, f.Name)
			}
			for _, name := range strings.Split(viewArg, ",") {
				name = strings.TrimSpace(name)
				f, ok := fileMap[name]
				if !ok {
					fmt.Printf("  %s %q not in artifact (available: %s)\n", cmdutil.WarningMark(), name, strings.Join(available, ", "))
					continue
				}
				fmt.Printf("# ── %s ──\n", name)
				data, err := client.ViewFile(cmd.Context(), ref, f)
				if err != nil {
					fmt.Printf("  error: %v\n", err)
					continue
				}
				fmt.Println(string(data))
			}
			return nil
		}

		deprecated := ""
		if m.Deprecated != nil {
			cmdutil.PrintPatternDeprecation(m.Deprecated)
			dep := m.Deprecated
			d := &types.CatalogDeprecation{
				MigratedTo: dep.MigratedTo,
				Message:    dep.Message,
			}
			if dep.TimelineFrom != "" || dep.TimelineTo != "" {
				d.Timeline = &types.DeprecationTimeline{
					From: dep.TimelineFrom,
					To:   dep.TimelineTo,
				}
			}
			switch d.DeprecationState(time.Now()) {
			case "eol":
				deprecated = " ← " + cmdutil.Red("✗") + " EOL"
			default:
				deprecated = " ← " + cmdutil.Yellow("⚠") + " deprecated"
			}
		}
		fmt.Printf("\n%s:%s\n", m.Name, m.Version)
		fmt.Printf("  Registry:    %s\n", ref.Registry)
		if m.Kind != "" {
			fmt.Printf("  Kind:        %s\n", m.Kind)
		}
		fmt.Printf("  Digest:      %s\n", info.Digest)
		if !info.PushedAt.IsZero() {
			fmt.Printf("  Pushed:      %s\n", info.PushedAt.Format(time.RFC3339))
		}
		fmt.Printf("  Size:        %s\n", cmdutil.FormatSize(info.Size))
		fmt.Printf("\n  Description: %s\n", cmdutil.WordWrap(m.Description, 55, "               "))
		if len(m.Tags) > 0 {
			fmt.Printf("  Tags:        %s\n", strings.Join(m.Tags, ", "))
		}
		if m.Author != "" {
			fmt.Printf("  Author:      %s\n", m.Author)
		}
		if m.License != "" {
			fmt.Printf("  License:     %s\n", m.License)
		}
		if m.Simulate != nil {
			switch m.Simulate.Status {
			case "passed":
				var suffix string
				if m.Simulate.Assertions == 1 {
					suffix = "1 assertion"
				} else if m.Simulate.Assertions > 1 {
					suffix = fmt.Sprintf("%d assertions", m.Simulate.Assertions)
				}
				if m.Simulate.Duration != "" {
					if suffix != "" {
						suffix += " · "
					}
					suffix += m.Simulate.Duration
				}
				if m.Simulate.TestedAt != "" {
					if t, err := time.Parse(time.RFC3339, m.Simulate.TestedAt); err == nil {
						if suffix != "" {
							suffix += " · "
						}
						suffix += "tested " + cmdutil.HumanDuration(time.Since(t)) + " ago"
					}
				}
				fmt.Printf("  Simulate:    %s\n", cmdutil.SimulateVerified(suffix))
			case "skipped":
				fmt.Printf("  Simulate:    %s\n", cmdutil.SimulateSkipped())
			case "no-assertion":
				fmt.Printf("  Simulate:    %s\n", cmdutil.SimulateNoAssertion())
			}
		}
		if m.Kind != registry.ModuleKind {
			if m.E2E != nil {
				switch m.E2E.Status {
				case "passed":
					var suffix string
					if m.E2E.Assertions == 1 {
						suffix = "1 assertion"
					} else if m.E2E.Assertions > 1 {
						suffix = fmt.Sprintf("%d assertions", m.E2E.Assertions)
					}
					if m.E2E.Duration != "" {
						if suffix != "" {
							suffix += " · "
						}
						suffix += m.E2E.Duration
					}
					if m.E2E.TestedAt != "" {
						if t, err := time.Parse(time.RFC3339, m.E2E.TestedAt); err == nil {
							if suffix != "" {
								suffix += " · "
							}
							suffix += "tested " + cmdutil.HumanDuration(time.Since(t)) + " ago"
						}
					}
					fmt.Printf("  E2E:         %s\n", cmdutil.E2eVerified(suffix))
				case "skipped":
					fmt.Printf("  E2E:         %s\n", cmdutil.E2eSkipped())
				}
			} else {
				fmt.Printf("  E2E:         %s\n", cmdutil.E2eNotVerified())
			}
		}
		if m.Intent != nil {
			switch m.Intent.Status {
			case "passed":
				var suffix string
				if m.Intent.Target != "" {
					suffix = "target: " + m.Intent.Target
				}
				if m.Intent.TestedAt != "" {
					if t, err := time.Parse(time.RFC3339, m.Intent.TestedAt); err == nil {
						if suffix != "" {
							suffix += " · "
						}
						suffix += "tested " + cmdutil.HumanDuration(time.Since(t)) + " ago"
					}
				}
				fmt.Printf("  Intent:      %s\n", cmdutil.Green("✓ passed"+(func() string {
					if suffix != "" {
						return " · " + suffix
					}
					return ""
				})()))
			case "failed":
				fmt.Printf("  Intent:      %s\n", cmdutil.Red("✗ failed"))
			}
		}
		if m.Typed != nil {
			parts := []string{}
			if m.Typed.HasHooks {
				parts = append(parts, "hooks")
			}
			if m.Typed.HasConstructor {
				parts = append(parts, "constructor")
			}
			fmt.Printf("  Typed:       %s · %s\n",
				cmdutil.Green("✓ "+strings.Join(parts, ", ")),
				cmdutil.Yellow("requires custom runtime image"),
			)
		}
		if m.RuntimeVersion != "" {
			fmt.Printf("  Runtime:     %s\n", m.RuntimeVersion)
		}
		if len(info.Files) > 0 {
			fmt.Printf("\n  Files:\n")
			for _, f := range info.Files {
				fmt.Printf("    %-30s %s\n", f.Name, cmdutil.FormatSize(f.Size))
			}
		}
		fmt.Printf("\nTo pull:\n")
		if m.Kind == registry.ModuleKind {
			fmt.Printf("  inrun pull %s:%s --module %s\n", m.Name, m.Version, deprecated)
		} else {
			fmt.Printf("  inrun pull %s:%s %s\n", m.Name, m.Version, deprecated)
		}
		fmt.Printf("\nTo import:\n")
		if m.Kind == registry.ModuleKind {
			fmt.Printf("  imports:\n")
			fmt.Printf("    - module: %s %s\n", ref.String(), deprecated)
		} else {
			fmt.Printf("  imports:\n")
			fmt.Printf("    registry:\n")
			fmt.Printf("      - %s %s\n", ref.String(), deprecated)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	inspectCmd.Flags().BoolVarP(&inspectModule, "module", "m", false, "Resolve as a module (uses INRUN_MODULES_REGISTRY)")
	inspectCmd.Flags().String("view", "", "Comma-separated list of files to print before pulling (e.g. catalog.yaml,cr.yaml)")
	inspectCmd.Flags().Bool("versions", false, "List up to 10 tracked versions with simulate and E2E status")
	cmdutil.RootCmd.AddCommand(inspectCmd)

	// Shadow global flags
	cmdutil.ShadowGlobalCommandFlags(inspectCmd, "file")
}
