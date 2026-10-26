package kubectl

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// runOptions is the unexported executor input: the argv, the directory it
// runs in, and the env added to kubectl's process environment. The args are
// used as given: an expectation's ${NAME} is resolved before it runs (see
// types.ExpandEnv). The zero Env adds nothing, so callers that don't need
// declared env can omit it.
type runOptions struct {
	args    []string
	workDir string
	env     types.EnvResolver
}

// run executes kubectl with args in workDir, adding env's declared values to
// its process environment.
func run(ctx context.Context, o runOptions) (string, error) {
	return runCommand(ctx, o, nil)
}

// runWithStdin is run plus a reader on the command's stdin, for
// `apply -f -`. The reader is passed through unchanged: expansion of the
// streamed content is the caller's decision, so a pre-expanded manifest and a
// binary both work.
func runWithStdin(ctx context.Context, o runOptions, stdin io.Reader) (string, error) {
	return runCommand(ctx, o, stdin)
}

func runCommand(ctx context.Context, o runOptions, stdin io.Reader) (string, error) {
	// The dry-run rewrite comes before the command is built: the command
	// copies its arguments.
	args := o.args
	if dryRun {
		args = applyDryRun(args)
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)

	if o.workDir != "" {
		cmd.Dir = o.workDir
	}

	cmd.Env = append(os.Environ(), o.env.Pairs()...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// applyDryRun rewrites args for a dry run: mutating verbs get
// --dry-run=server, reads are left alone. Of rollout, only the subcommands
// that change the object take the flag: status and history are reads, and
// kubectl rejects --dry-run on them.
func applyDryRun(args []string) []string {
	if len(args) == 0 {
		return args
	}
	switch args[0] {
	case "apply", "patch", "delete", "scale":
		return append(args, "--dry-run=server")
	case "rollout":
		if len(args) > 1 {
			switch args[1] {
			case "restart", "undo", "pause", "resume":
				return append(args, "--dry-run=server")
			}
		}
	}
	return args
}

// dryRun, when set, makes run append --dry-run=server to mutating verbs
// (apply, patch, delete, scale, restart, …) and skip execution entirely for
// reads, which a dry run cannot change. Set by the E2E runner when
// --dry-run is passed; unset by default.
var dryRun bool

// SetDryRun toggles dry-run mode for every subsequent kubectl invocation in
// this process. The E2E runner sets it once at startup, from --dry-run.
func SetDryRun(v bool) { dryRun = v }

// DryRun reports whether dry-run mode is on. Tests use it to assert the flag
// without reading package state.
func DryRun() bool { return dryRun }

// CurrentContext returns the kubeconfig's current context.
func CurrentContext(ctx context.Context) (string, error) {
	return run(ctx, runOptions{args: []string{"config", "current-context"}})
}

// UseContext switches the kubeconfig's current context to name.
func UseContext(ctx context.Context, name string) (string, error) {
	return run(ctx, runOptions{args: []string{"config", "use-context", name}})
}
