//go:build !runtime && !gateway

package testsuite

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/registry/simulate"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// compareOne simulates one CR in memory and on envtest from the same input,
// and prints each run and their comparison. The simulate package runs,
// judges and compares; this sequences the two runs — each attempted
// whatever the other did — and prints between them.
func compareOne(
	ctx context.Context,
	opts simulate.RunOptions,
	simOpts CliSimulateOptions,
	crdPaths []string,
	expect *types.SimulateExpect,
	resolver *template.Resolver,
) (*simulate.Result, error) {
	if opts.MaxCycles == 0 {
		opts.MaxCycles = 10
	}
	opts.CRDPaths = crdPaths
	opts.K8sVersion = simOpts.K8sVersion
	printSimulateHeader(opts, simOpts, "compare")

	runOn := func(heading string, backend simulate.Backend) (*simulate.Result, error) {
		fmt.Printf("  %s\n", heading)
		runOpts := opts.Isolated()
		res, err := underSpinner(fmt.Sprintf("  Running %d cycles...", opts.MaxCycles), simOpts.Verbose, func() (*simulate.Result, error) {
			return simulate.New(ctx, runOpts).Execute(backend)
		})
		if err != nil {
			fmt.Printf("  %s %v\n\n", cmdutil.FailureMark(), err)
			return nil, err
		}
		printNotes(res)
		if n := res.CycleErrors(); n > 0 {
			fmt.Printf("  %s %d cycle(s) returned an error\n", cmdutil.FailureMark(), n)
		}
		printSteady(res, opts.MaxCycles)
		errs, evalErr := simulate.Evaluate(res, expect, resolver, runOpts.CR)
		printAssertionSummary(res, errs, expect, evalErr)
		return res, nil
	}

	mem, memErr := runOn("In-memory", simulate.InMemory)
	env, envErr := runOn("Envtest", simulate.Envtest)

	cmp := simulate.Compare(opts.CRDName, mem, env, memErr, envErr)
	printComparison(cmp, simOpts.Verbose)

	err := cmp.Err()
	diff := cmp.Differences()
	diffTxt := "difference"
	if diff > 1 {
		diffTxt = "differences"
	}
	switch {
	case err == nil:
		fmt.Printf("  %s\n\n", cmdutil.Green("PASS — simulations equivalent"))
	case cmp.Complete() && diff > 0:
		fmt.Printf("  %s\n\n", cmdutil.Red(fmt.Sprintf("FAIL — %d %s found", diff, diffTxt)))
	default:
		fmt.Printf("  %s\n\n", cmdutil.Red("FAIL — "+err.Error()))
	}
	return cmp.Result(), err
}
