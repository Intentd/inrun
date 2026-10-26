package expect

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"gopkg.in/yaml.v3"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

// Run executes the configured expectations in order.
// It applies or deletes CRs when first required, resolves expectation
// templates, records results, and runs each expectation's onFailure and
// finally post blocks. It stops when an expectation requests it or the
// maximum failure count is reached.
type Options struct {
	CRFiles     []string
	WorkDir     string
	NoRuntime   bool
	MaxFailures int

	CS      kubernetes.Interface
	Cfg     *rest.Config
	Dynamic dynamic.Interface
	Mapper  apimeta.RESTMapper
	Spec    *types.E2ESpec
}

const defaultTimeout = "60s"

// Run executes the configured expectations in order.
// It applies or deletes CRs when first required, resolves expectation
// templates, records results, and handles onFailure/finally actions.
// It stops when an expectation requests it or the maximum failure
// count is reached.
func Run(ctx context.Context, expects types.E2EExpectations, opts Options) (
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
		// Resolve the expectation's env once, as the template resolver
		// resolves a source: what runs below sees final values, and a name
		// nothing declares fails the case before it runs.
		env := opts.Spec.ResolveEnv(exp)
		unknownEnv := types.UnknownEnv(env, exp)
		exp = types.ExpandEnv(env, exp)
		expResolver := resolver.WithEnv(env.Map())
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
					if out, err := kubectl.Apply(ctx, kubectl.ApplyOptions{File: p, Env: env}); err != nil {
						return nil, failures, stopped, stopReason,
							fmt.Errorf("apply CR %s: %w\n%s", p, err, out)
					}
				}
				fmt.Printf("  %s CR(s) applied\n\n", successMark())
				crApplied = true
			}

		case types.AfterCRDeleted:
			// The delete doesn't wait for finalizers: the expectations after
			// cr-deleted wait for what the deletion should bring about, each
			// within its own timeout.
			if !crDeleted && !opts.NoRuntime {
				fmt.Printf("→ Deleting CR(s)...\n")
				for _, p := range opts.CRFiles {
					if out, err := kubectl.Delete(ctx, kubectl.DeleteOptions{File: p, Env: env, IgnoreNotFound: true, Wait: ptr.To(false)}); err != nil {
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

		workDir := exp.Dir(opts.WorkDir)

		sc := types.E2EExpectationScope{
			Expectation: exp,
			WorkDir:     workDir,
			CS:          opts.CS,
			Cfg:         opts.Cfg,
			Dynamic:     opts.Dynamic,
			Mapper:      opts.Mapper,
			Env:         env,
			CRData:      loadCRData(spec, opts.WorkDir),
		}
		var vr StepResult
		var verifyErr error
		if len(unknownEnv) > 0 {
			vr = StepResult{Configured: 1 + max(exp.Retry, 0)}
			verifyErr = fmt.Errorf("unknown env %s: declare it in env: or set it in the environment",
				"${"+strings.Join(unknownEnv, "}, ${")+"}")
		} else {
			vr, verifyErr = runStep(ctx, sc, expResolver)
		}
		close(waitDone)
		caseElapsed := time.Since(caseStart)

		skipped := verifyErr == errSkipped
		cres := CaseResult{
			Name:     exp.Name,
			Passed:   verifyErr == nil,
			Skipped:  skipped,
			Elapsed:  caseElapsed,
			Err:      verifyErr,
			Attempts: vr.Attempts,
			Retries:  vr.Configured - 1,
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
			// A dry run changed nothing, so there is nothing to diagnose or
			// clean up; onFailure and finally run shell the dry run cannot reach.
			if !kubectl.DryRun() && exp.OnFailure != nil {
				cres := &cases[len(cases)-1]
				cres.Warnings = append(cres.Warnings,
					runPost(ctx, "onFailure", &exp.OnFailure.E2ESideEffects, sc, expResolver)...)
			}
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

		if !kubectl.DryRun() && exp.Finally != nil {
			cres := &cases[len(cases)-1]
			cres.Warnings = append(cres.Warnings,
				runPost(ctx, "finally", &exp.Finally.E2ESideEffects, sc, expResolver)...)
		}
		if stopped != nil {
			break expectations
		}
	}

	return cases, failures, stopped, stopReason, nil
}

// loadCRData reads spec.CR and unmarshals it into a map for condition evaluation.
// Returns nil when spec is nil, CR is unset, or the file cannot be read.
func loadCRData(spec *types.E2ESpec, workDir string) map[string]interface{} {
	if spec == nil || spec.CR == "" {
		return nil
	}
	crPath := spec.CR
	if !filepath.IsAbs(crPath) {
		crPath = filepath.Join(workDir, crPath)
	}
	data, err := readLocal(crPath)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	_ = yaml.Unmarshal(data, &out)
	return out
}
