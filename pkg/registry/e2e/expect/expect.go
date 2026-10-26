package expect

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Run executes the configured expectations in order.
// It applies or deletes CRs when first required, resolves expectation
// templates, records results, and handles onFailure/finally actions.
// It stops when an expectation requests it or the maximum failure
// count is reached.
type Options struct {
	CRFiles     []string
	WorkDir     string
	NoRuntime   bool
	MaxFailures int

	CS   kubernetes.Interface
	Cfg  *rest.Config
	Spec *types.E2ESpec
}

const defaultTimeout = "60s"

// Run executes the configured expectations in order.
// It applies or deletes CRs when first required, resolves expectation
// templates, records results, and handles onFailure/finally actions.
// It stops when an expectation requests it or the maximum failure
// count is reached.
func Run(ctx context.Context, expects []types.E2EExpectation, opts Options) (
	[]CaseResult, int, error, string, error,
) {

	spec := opts.Spec
	resolver := template.NewResolverFromMap(map[string]interface{}{})
	if spec != nil {
		resolver = template.NewResolverWithFunc(spec.Functions)
	}

	var (
		cases      []CaseResult
		failures   int
		stopped    error
		stopReason string
		crApplied  bool
		crDeleted  bool
	)

expectations:
	for _, exp := range expects {
		env := opts.Spec.ResolveEnv(exp)
		after := exp.After
		if after == "" {
			after = types.AfterSetupComplete
		}

		switch after {
		case types.AfterSetupComplete:
			// Infrastructure assertions — no CR lifecycle action needed.

		case types.AfterCRApplied:
			if !crApplied && !opts.NoRuntime {
				fmt.Printf("→ Applying CR(s)...\n")
				for _, p := range opts.CRFiles {
					if out, err := runKubectl(ctx, "", env, "apply", "-f", p); err != nil {
						return nil, failures, stopped, stopReason,
							fmt.Errorf("apply CR %s: %w\n%s", p, err, out)
					}
				}
				fmt.Printf("  %s CR(s) applied\n\n", successMark())
				crApplied = true
			}

		case types.AfterCRDeleted:
			if !crDeleted && !opts.NoRuntime {
				fmt.Printf("→ Deleting CR(s)...\n")
				for _, p := range opts.CRFiles {
					if out, err := runKubectl(ctx, "", env, "delete", "-f", p, "--ignore-not-found"); err != nil {
						return nil, failures, stopped, stopReason,
							fmt.Errorf("delete CR %s: %w\n%s", p, err, out)
					}
				}
				fmt.Printf("  %s CR(s) deleted\n\n", successMark())
				crDeleted = true
			}

		default:
			return nil, failures, stopped, stopReason,
				fmt.Errorf("unknown after: %q — valid values: %v", after, types.ValidAfterValues)
		}

		to := exp.Timeout
		if to == "" {
			to = defaultTimeout
		}
		fmt.Printf("  Waiting for %q (timeout: %s)...\n", exp.Name, to)

		caseStart := time.Now()
		waitDone := make(chan struct{})
		go func() {
			t := time.NewTicker(10 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-waitDone:
					return
				case <-t.C:
					fmt.Printf("    still waiting [%s]...\n", time.Since(caseStart).Round(time.Second))
				}
			}
		}()

		workDir := opts.WorkDir
		if exp.WorkDir != "" {
			workDir = exp.WorkDir
		}

		verifyErr := verify(ctx, types.E2EExpectationScope{
			Expectation: exp,
			WorkDir:     workDir,
			CS:          opts.CS,
			Cfg:         opts.Cfg,
			Env:         env,
		}, resolver)
		close(waitDone)
		caseElapsed := time.Since(caseStart)

		skipped := verifyErr == errSkipped
		cres := CaseResult{
			Name:    exp.Name,
			Passed:  verifyErr == nil,
			Skipped: skipped,
			Elapsed: caseElapsed,
			Err:     verifyErr,
		}
		if skipped {
			cres.Err = nil
		}
		cases = append(cases, cres)

		switch {
		case skipped:
			fmt.Printf("  ~ %s (skipped)\n", exp.Name)

		case verifyErr != nil:
			fmt.Printf("  %s %s (%s): %v\n",
				failureMark(), exp.Name, caseElapsed.Round(time.Millisecond), verifyErr)
			runOnFailure(ctx, exp.OnFailure, workDir, opts.CS)
			failures++

			if exp.EffectiveStop(spec) {
				stopped, stopReason = errMaxFailures, "step"
			} else if opts.MaxFailures > 0 && failures >= opts.MaxFailures {
				stopped, stopReason = errMaxFailures, "max-failures"
			}

		default:
			fmt.Printf("  %s %s (%s)\n",
				successMark(), exp.Name, caseElapsed.Round(time.Millisecond))
		}

		runFinally(ctx, exp.Finally, workDir, opts.CS)
		if stopped != nil {
			break expectations
		}
	}

	return cases, failures, stopped, stopReason, nil
}
