package expect

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// postTimeout bounds one post block. A post block runs on a context the run's
// cancellation does not reach, so an interrupt or a suite timeout still
// cleans up; the bound keeps a hung cleanup from holding the run forever.
var postTimeout = 30 * time.Second

// runPost runs a post block — onFailure: or finally: — named by its key. It
// runs a step's operations with a different policy: each once, in order,
// never stopping early, printing each one's output. It never fails: it
// returns a warning for each operation that failed or whose assertions did
// not hold, for the case to carry.
func runPost(ctx context.Context, name string, block *types.E2ESideEffects, sc types.E2EExpectationScope, resolver *template.Resolver) []string {
	if block.Empty() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), postTimeout)
	defer cancel()

	fmt.Printf("\n── %s ──\n", name)
	var warnings []string
	report := func(label, out string, err error) {
		printPost(label, out, err)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s %s: %v", name, label, err))
		}
	}

	for i, c := range block.Commands {
		out, err := runCommand(ctx, c, sc.WorkDir, sc.Env)
		report(fmt.Sprintf("commands[%d]", i), out, err)
	}
	if block.Kubectl != nil {
		kubectl.Post(ctx, kubectl.BlockOptions{
			Kubectl: block.Kubectl,
			Env:     sc.Env,
			WorkDir: sc.WorkDir,
			Client:  sc.CS,
			Config:  sc.Cfg,
		}, report)
	}
	for i, c := range block.External {
		out, err := runExternal(ctx, c, sc.WorkDir, resolver, sc.CS)
		report(fmt.Sprintf("external[%d] %s", i, c.Name), out, err)
	}
	return warnings
}

// SuitePostOptions is what a spec-level post block needs: the spec, for its
// env and functions, and the cluster it runs against.
type SuitePostOptions struct {
	Spec    *types.E2ESpec
	WorkDir string
	CS      kubernetes.Interface
	Cfg     *rest.Config
}

// SuitePost runs a spec-level post block (the spec's onFailure or finally),
// named by its key, once for the suite, resolved the way an expectation is:
// spec.env expanded into it once and in its commands' environment,
// spec.functions in its templates. It returns the block's warnings; a name
// nothing declares is one, and the block does not run.
func SuitePost(ctx context.Context, name string, block *types.E2ESideEffects, o SuitePostOptions) []string {
	spec := o.Spec
	if spec == nil || block.Empty() {
		return nil
	}
	env := spec.ResolveEnv(types.E2EExpectation{})
	if unknown := types.UnknownEnv(env, block); len(unknown) > 0 {
		w := fmt.Sprintf("%s: unknown env %s: declare it in env: or set it in the environment",
			name, "${"+strings.Join(unknown, "}, ${")+"}")
		fmt.Printf("\n  ! %s\n", w)
		return []string{w}
	}
	expanded := types.ExpandEnv(env, *block)
	resolver := template.NewResolverWithFunc(spec.Functions).WithEnv(env.Map())
	return runPost(ctx, name, &expanded, types.E2EExpectationScope{
		WorkDir: o.WorkDir,
		CS:      o.CS,
		Cfg:     o.Cfg,
		Env:     env,
	}, resolver)
}

// printPost prints one post operation's output indented under its label, or
// its error when it printed nothing.
func printPost(label, out string, err error) {
	fmt.Printf("\n  ─ %s\n", label)
	out = strings.TrimSpace(out)
	if out != "" {
		for _, line := range strings.Split(out, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}
	if err != nil {
		fmt.Printf("  ! %s\n", strings.SplitN(err.Error(), "\n", 2)[0])
	}
}
