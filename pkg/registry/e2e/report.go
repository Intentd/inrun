package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/expect"
)

func (r *Runner) printReport(
	ctx context.Context,
	name string,
	cases []expect.CaseResult,
	result *expect.Result,
	isPureAgg bool,
) {
	if isPureAgg {
		return
	}

	fmt.Printf("\nE2E Results: %s\n\n", name)

	var failures []expect.CaseResult
	var skippedCases []expect.CaseResult

	for _, c := range cases {
		switch {
		case c.Passed:
			fmt.Printf("  %s %-40s (%s)\n", successMark(), c.Name, c.Elapsed.Round(time.Millisecond))
		case c.Skipped:
			skippedCases = append(skippedCases, c)
		default:
			failures = append(failures, c)
		}
	}

	if len(failures) > 0 {
		fmt.Println()
		for _, c := range failures {
			fmt.Printf("  %s %-40s (%s)\n", failureMark(), c.Name, c.Elapsed.Round(time.Millisecond))
			if c.Err != nil {
				for _, line := range strings.Split(strings.TrimSpace(c.Err.Error()), "\n") {
					fmt.Printf("      %s\n", line)
				}
			}
		}
	}

	if len(skippedCases) > 0 {
		fmt.Println()
		for _, c := range skippedCases {
			fmt.Printf("  ~ %-40s (skipped)\n", c.Name)
		}
	}

	clusterInfo := r.clusterName()
	fmt.Printf("\n  %s\n", result.Summary())
	if r.needsCluster {
		if clusterInfo != "" {
			fmt.Printf("  Cluster: %s (%s)\n", clusterInfo, r.provider())
		}
	}

	if len(failures) > 0 {
		runOnFailure(ctx, r.e2e.Spec.OnFailure, r.e2eDir, r.cs)
	}
}
