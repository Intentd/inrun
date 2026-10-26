//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/cmd/cli/serve"
	"github.com/intentd/inrun/cmd/cli/testsuite"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/registry/e2e"
	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/spf13/cobra"
)

// runPushGates runs the verification gates and returns their recorded metadata.
// Intent play remains informational: a failed play is recorded but does not block
// the push, matching the existing push behaviour.
func runPushGates(cmd *cobra.Command, dir string, supportsGates bool) (
	*registry.PatternSimulate,
	*registry.PatternE2E,
	*registry.PatternIntent,
	error,
) {
	var (
		simMeta    *registry.PatternSimulate
		e2eMeta    *registry.PatternE2E
		intentMeta *registry.PatternIntent

		simFailed bool
		e2eFailed bool
	)

	if supportsGates {
		// Simulate Gate
		if rel := registry.FindPatternFile(dir, registry.FileSimulate); rel != "" {
			simFile := filepath.Join(dir, rel)
			if pushForce || pushNoSimulate {
				fmt.Printf("  ~ Simulate %s\n", registry.StatusSkipped.String())
				simMeta = &registry.PatternSimulate{
					Status:   registry.StatusSkipped,
					TestedAt: time.Now().UTC().Format(time.RFC3339),
				}
			} else {
				hasAssertions := simulateFileHasAssertions(simFile)
				if !hasAssertions {
					fmt.Printf("  %s simulate.yaml has no assertions — add expect: to enforce behavior\n", cmdutil.Yellow("⚠"))
					simMeta = &registry.PatternSimulate{
						Status:   registry.StatusNoAssertion,
						TestedAt: time.Now().UTC().Format(time.RFC3339),
					}
				} else {
					fmt.Printf("\nRunning simulate gate (%s)...\n", registry.FileSimulate)
					start := time.Now()
					result, runErr := testsuite.RunSimulateFromSpec(cmd.Context(), simFile, testsuite.CliSimulateOptions{MaxCycles: 10})
					dur := time.Since(start).Round(time.Millisecond).String()

					simMeta = &registry.PatternSimulate{
						Duration:   dur,
						TestedAt:   time.Now().UTC().Format(time.RFC3339),
						Assertions: countSimulateAssertions(simFile),
						Passed:     result.Passed(),
						Failed:     result.Failed(),
					}

					if runErr == nil {
						simMeta.Status = registry.StatusPassed
						fmt.Printf("  %s Simulate passed (%s)\n", cmdutil.SuccessMark(), dur)
					} else {
						simMeta.Status = registry.StatusFailed
						fmt.Printf("  %s Simulate failed (%s)\n", cmdutil.FailureMark(), dur)

						if pushForce {
							fmt.Printf("  %s continuing (--force)\n", cmdutil.WarningMark())
						} else if !term.IsTerminal() {
							return nil, nil, nil, fmt.Errorf("Simulate gate failed — push blocked\n  Run 'inrun simulate' to see the failures\n  Use --force to override (recorded in the artifact)\n\n%w", runErr)
						} else {
							simFailed = true
						}
					}
				}
			}
		}

		// E2E Gate
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
				fmt.Printf("  ~ E2E %s\n", registry.StatusSkipped.String())
				e2eMeta = &registry.PatternE2E{
					Status:   registry.StatusSkipped,
					TestedAt: time.Now().UTC().Format(time.RFC3339),
					Runner:   detectRunner(),
				}
			} else {
				hasAssertions := e2eFileHasAssertions(e2eFile)
				if !hasAssertions {
					fmt.Printf("  %s e2e.yaml has no assertions — add expect: to enforce behavior\n", cmdutil.Yellow("⚠"))
					e2eMeta = &registry.PatternE2E{
						Status:   registry.StatusNoAssertion,
						TestedAt: time.Now().UTC().Format(time.RFC3339),
					}
				} else {

					chart, err := cmdutil.ChartDir(pushE2EChart)
					if err != nil {
						return nil, nil, nil, err
					}
					if pushE2EUseCurrent {
						if !cluster.ClusterReachable() {
							return nil, nil, nil, fmt.Errorf("Cannot reach a Kubernetes cluster. " +
								"\n  Check your kubeconfig, or run without '--use-current'.")
						}
					}

					runner, err := e2e.New(e2eFile, e2e.Options{
						ClusterCtx: pushE2ECluster, UseCurrentCtx: pushE2EUseCurrent, Workers: pushE2EWorkers,
						InrunVersion: pushE2EVersion, InrunChart: chart, ValueFiles: pushE2EValues,
						HelmArgs: cmdutil.HelmSetArgs(pushE2ESet), ReportFile: cmdutil.FileReportJSON,
						PushTriggered: true,
					})
					if err != nil {
						return nil, nil, nil, fmt.Errorf("e2e gate: %w\n\nUse --force or --no-e2e to skip", err)
					}

					fmt.Printf("\nRunning E2E gate (%s)...\n", registry.FileE2E)
					result, runErr := runner.Run(cmd.Context())

					e2eMeta = &registry.PatternE2E{
						Duration:   result.Duration(),
						TestedAt:   time.Now().UTC().Format(time.RFC3339),
						Runner:     detectRunner(),
						Assertions: result.Total(),
						Passed:     result.Passed(),
						Failed:     result.Failed(),
						Skipped:    result.Skipped(),
					}

					if runErr == nil {
						e2eMeta.Status = registry.StatusPassed
						fmt.Printf("  %s E2E passed (%s)\n", cmdutil.SuccessMark(), result.Duration())
					} else {
						e2eMeta.Status = registry.StatusFailed
						fmt.Printf("  %s E2E failed — %s\n", cmdutil.FailureMark(), result.Summary())

						if pushForce {
							fmt.Printf("  %s continuing (--force)\n", cmdutil.WarningMark())
						} else if !term.IsTerminal() {
							return nil, nil, nil, fmt.Errorf("E2E gate failed — push blocked\n  Use --force to override (recorded in the artifact)\n\n%w", runErr)
						} else {
							e2eFailed = true
						}
					}
				}

			}
		}

		// One prompt for one decision: push with failures recorded, or abort.
		if simFailed || e2eFailed {
			if pushForce {
				// non-interactive: proceed, both failures are recorded
			} else if !term.IsTerminal() {
				return nil, nil, nil, fmt.Errorf(
					"gates failed (simulate: %v, e2e: %v) — push blocked\n"+
						"  Run 'inrun simulate' / 'inrun e2e' to see the failures\n"+
						"  Use --force to override (recorded in the artifact)",
					simFailed, e2eFailed)
			} else {
				fmt.Printf("\n  simulate: %s\n", gateLine(simMeta))
				fmt.Printf("\n  e2e:      %s\n\n", gateLine(e2eMeta))
				ok, err := term.Confirm("Push anyway?")
				if err != nil {
					return nil, nil, nil, fmt.Errorf("push with --force: %w", err)
				}
				if !ok {
					return nil, nil, nil, fmt.Errorf("push aborted")
				}
			}
		}
		// --no-e2e was explicitly passed but no e2e.yaml exists; record as skipped
		// so inrun inspect shows Skipped rather than - Not verified.
		if pushNoE2E && e2eMeta == nil {
			fmt.Printf("  ~ E2E %s\n", registry.StatusSkipped.String())
			e2eMeta = &registry.PatternE2E{
				Status:   registry.StatusSkipped,
				TestedAt: time.Now().UTC().Format(time.RFC3339),
				Runner:   detectRunner(),
			}
		}
		// Intent play
		if pushAddIntent != "" {
			intentFile := pushAddIntent
			if !filepath.IsAbs(intentFile) {
				intentFile = filepath.Join(dir, intentFile)
			}
			fmt.Printf("\nRunning intent play (%s)...\n", pushAddIntent)
			target, perr := serve.RunIntentPlay(filepath.Join(dir, cmdutil.FileBlueprint), intentFile)
			status := registry.StatusPassed
			if perr != nil {
				status = registry.StatusFailed
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
	}
	return simMeta, e2eMeta, intentMeta, nil
}

// gateLine renders one gate's status for the push recap:
//
//	✓ passed · 2s
//	✗ failed · 2 of 30 · 28s
//	- not verified
func gateLine(g registry.GateStatus) string {
	if g == nil || g.GateStatus() == "" {
		return "- not verified"
	}
	st := g.GateStatus()
	line := st.Symbol() + " " + st.String()
	if failed, total := g.GateCounts(); failed > 0 && total > 0 {
		line += fmt.Sprintf(" · %d of %d", failed, total)
	}
	if d := g.GateDuration(); d != "" {
		line += " · " + d
	}
	return line
}
