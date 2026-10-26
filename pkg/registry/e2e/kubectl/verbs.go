package kubectl

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// ApplyOptions is one `kubectl apply -f <file> [-n <ns>]`.
type ApplyOptions struct {
	File      string
	Namespace string
	WorkDir   string
	Env       types.EnvResolver
}

// Apply runs kubectl apply -f <file>.
func Apply(ctx context.Context, o ApplyOptions) (string, error) {
	args := []string{"apply", "-f", o.File}
	if o.Namespace != "" {
		args = append(args, "-n", o.Namespace)
	}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// ApplyInlineOptions is one `kubectl apply -f - [-n <ns>]` fed through stdin.
type ApplyInlineOptions struct {
	Content   string
	Namespace string
	WorkDir   string
	Env       types.EnvResolver
}

// ApplyInline runs kubectl apply -f - with Content on stdin. Content arrives
// resolved: an inline manifest is part of its expectation, whose ${NAME} is
// expanded before it runs.
func ApplyInline(ctx context.Context, o ApplyInlineOptions) (string, error) {
	args := []string{"apply", "-f", "-"}
	if o.Namespace != "" {
		args = append(args, "-n", o.Namespace)
	}
	return runWithStdin(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env},
		strings.NewReader(o.Content))
}

// PatchOptions is one `kubectl patch <kind> <name> -n <ns> --type <type>`
// with -p <patch> or --patch-file <file>.
type PatchOptions struct {
	Kind      string
	Name      string
	Namespace string
	Type      string // "merge" (default), "strategic", "json"
	Patch     string
	PatchFile string // used when Patch is empty
	WorkDir   string
	Env       types.EnvResolver
}

// Patch runs kubectl patch.
func Patch(ctx context.Context, o PatchOptions) (string, error) {
	typ := o.Type
	if typ == "" {
		typ = "merge"
	}
	args := append([]string{"patch", o.Kind, o.Name, "-n", defaultOr(o.Namespace), "--type", typ}, patchArgs(o.Patch, o.PatchFile)...)
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// PatchFileOptions is one `kubectl patch -f <file> --type <type>` with
// -p <patch> or --patch-file <file>.
type PatchFileOptions struct {
	File      string
	Type      string // "merge" (default), "strategic", "json"
	Patch     string
	PatchFile string // used when Patch is empty
	WorkDir   string
	Env       types.EnvResolver
}

// PatchFile runs kubectl patch -f <file>, the manifest form of patch. It
// patches every object in File, so it has no -n: kubectl reads the namespace
// from each document.
func PatchFile(ctx context.Context, o PatchFileOptions) (string, error) {
	typ := o.Type
	if typ == "" {
		typ = "merge"
	}
	args := append([]string{"patch", "-f", o.File, "--type=" + typ}, patchArgs(o.Patch, o.PatchFile)...)
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// patchArgs passes the patch content inline, or from a file.
func patchArgs(patch, file string) []string {
	if patch == "" && file != "" {
		return []string{"--patch-file", file}
	}
	return []string{"-p", patch}
}

// GetOptions is one `kubectl get <kind> [name] [-l <selector>] -n <ns> [-o <format>]`
// or, with AllNS, `kubectl get <kind> --all-namespaces`.
type GetOptions struct {
	Kind           string
	Name           string // mutually exclusive with LabelSelector
	LabelSelector  string
	AllNamespaces  bool // --all-namespaces; suppresses -n
	Namespace      string
	Format         string
	WorkDir        string
	Env            types.EnvResolver
	IgnoreNotFound bool // --ignore-not-found
}

// Get runs kubectl get.
func Get(ctx context.Context, o GetOptions) (string, error) {
	args := []string{"get", o.Kind}
	if o.Name != "" {
		args = append(args, o.Name)
	} else if o.LabelSelector != "" {
		args = append(args, "-l", o.LabelSelector)
	}
	if o.AllNamespaces {
		args = append(args, "--all-namespaces")
	} else {
		args = append(args, "-n", defaultOr(o.Namespace))
	}
	if o.Format != "" {
		args = append(args, "-o", o.Format)
	}
	if o.IgnoreNotFound {
		args = append(args, "--ignore-not-found")
	}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// DeleteOptions is one `kubectl delete -f <file>` or
// `kubectl delete <kind> <name> -n <ns>`.
type DeleteOptions struct {
	File           string
	Kind           string
	Name           string
	Namespace      string
	IgnoreNotFound bool
	Timeout        string // --timeout=<dur>; empty means kubectl's default
	Wait           *bool  // --wait=false when set to false; nil means kubectl's default (wait)
	WorkDir        string
	Env            types.EnvResolver
}

// Delete runs kubectl delete.
func Delete(ctx context.Context, o DeleteOptions) (string, error) {
	var args []string
	if o.File != "" {
		args = []string{"delete", "-f", o.File}
	} else {
		args = []string{"delete", o.Kind, o.Name, "-n", defaultOr(o.Namespace)}
	}
	if o.IgnoreNotFound {
		args = append(args, "--ignore-not-found")
	}
	if o.Timeout != "" {
		args = append(args, "--timeout="+o.Timeout)
	}
	if o.Wait != nil && !*o.Wait {
		args = append(args, "--wait=false")
	}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// LogsOptions is one `kubectl logs -n <ns> <target> [-c <container>] [--since=<d>]`.
type LogsOptions struct {
	Target        string // pod name; mutually exclusive with LabelSelector
	LabelSelector string // -l <selector>
	Namespace     string
	Container     string
	Since         string
	WorkDir       string
	Env           types.EnvResolver
}

// Logs runs kubectl logs.
func Logs(ctx context.Context, o LogsOptions) (string, error) {
	args := []string{"logs", "-n", defaultOr(o.Namespace)}
	if o.LabelSelector != "" {
		// Every pod's whole log: with a selector kubectl keeps only the last 10 lines of each
		args = append(args, "-l", o.LabelSelector, "--tail=-1")
	} else if o.Target != "" {
		args = append(args, o.Target)
	}
	if o.Container != "" {
		args = append(args, "-c", o.Container)
	}
	if o.Since != "" {
		args = append(args, "--since="+o.Since)
	}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// ExecOptions is one `kubectl exec -n <ns> <pod> [-- <command…>]`.
type ExecOptions struct {
	Pod       string
	Namespace string
	Container string
	Command   []string // argv after --
	WorkDir   string
	Env       types.EnvResolver
}

// Exec runs kubectl exec.
func Exec(ctx context.Context, o ExecOptions) (string, error) {
	args := []string{"exec", "-n", defaultOr(o.Namespace), o.Pod}
	if o.Container != "" {
		args = append(args, "-c", o.Container)
	}
	args = append(args, "--")
	args = append(args, o.Command...)
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// DescribeOptions is one `kubectl describe <kind> [name|-l selector] -n <ns>`.
type DescribeOptions struct {
	Kind          string
	Name          string // mutually exclusive with LabelSelector
	LabelSelector string
	Namespace     string
	WorkDir       string
	Env           types.EnvResolver
}

// Describe runs kubectl describe.
func Describe(ctx context.Context, o DescribeOptions) (string, error) {
	args := []string{"describe", o.Kind, "-n", o.Namespace}
	if o.Name != "" {
		args = append(args, o.Name)
	} else if o.LabelSelector != "" {
		args = append(args, "-l", o.LabelSelector)
	}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// EventsOptions is one `kubectl events --for <kind>/<name> -n <ns>`.
type EventsOptions struct {
	Kind      string
	Name      string
	Namespace string
	WorkDir   string
	Env       types.EnvResolver
}

// Events runs kubectl events.
func Events(ctx context.Context, o EventsOptions) (string, error) {
	args := []string{"events", "--for", o.Kind + "/" + o.Name, "-n", defaultOr(o.Namespace)}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// CpOptions is one `kubectl cp <src> <dst> [-c <container>]`.
type CpOptions struct {
	Source    string // "ns/pod:/path/inside"
	Dest      string // local path
	Container string
	WorkDir   string
	Env       types.EnvResolver
}

// Cp runs kubectl cp.
func Cp(ctx context.Context, o CpOptions) (string, error) {
	args := []string{"cp", o.Source, o.Dest}
	if o.Container != "" {
		args = append(args, "-c", o.Container)
	}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// TopOptions is one `kubectl top pod|node [name|-l selector] [-n <ns>]
// [--containers]`.
type TopOptions struct {
	Kind          string // "pod"/"pods" or "node"/"nodes"
	Name          string
	LabelSelector string
	Namespace     string
	Containers    bool
	WorkDir       string
	Env           types.EnvResolver
}

// Top runs kubectl top.
func Top(ctx context.Context, o TopOptions) (string, error) {
	kind := strings.ToLower(o.Kind)
	args := []string{"top", kind}

	if kind == "pod" || kind == "pods" {
		args = append(args, "-n", defaultOr(o.Namespace))
		if o.LabelSelector != "" {
			args = append(args, "-l", o.LabelSelector)
		} else if o.Name != "" {
			args = append(args, o.Name)
		}
		if o.Containers {
			args = append(args, "--containers")
		}
	} else if o.Name != "" {
		args = append(args, o.Name)
	}

	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// RestartOptions is one `kubectl rollout restart <target> -n <ns>`.
type RestartOptions struct {
	Target    string // "deployment/name", "statefulset/name", …
	Namespace string
	WorkDir   string
	Env       types.EnvResolver
}

// Restart runs kubectl rollout restart.
func Restart(ctx context.Context, o RestartOptions) (string, error) {
	args := []string{"rollout", "restart", o.Target, "-n", defaultOr(o.Namespace)}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// RolloutSub is a kubectl rollout subcommand: restart, status, undo, …
type RolloutSub string

const (
	RolloutRestart RolloutSub = "restart"
	RolloutStatus  RolloutSub = "status"
)

// RolloutOptions is one `kubectl rollout <sub> <target> -n <ns>`.
type RolloutOptions struct {
	Sub       RolloutSub
	Target    string
	Namespace string
	WorkDir   string
	Env       types.EnvResolver
}

// Rollout runs kubectl rollout.
func Rollout(ctx context.Context, o RolloutOptions) (string, error) {
	args := []string{"rollout", string(o.Sub), o.Target, "-n", defaultOr(o.Namespace)}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}

// ScaleOptions is one `kubectl scale <target> --replicas=<n> -n <ns>`.
type ScaleOptions struct {
	Target    string
	Replicas  int
	Namespace string
	WorkDir   string
	Env       types.EnvResolver
}

// Scale runs kubectl scale.
func Scale(ctx context.Context, o ScaleOptions) (string, error) {
	args := []string{"scale", o.Target, fmt.Sprintf("--replicas=%d", o.Replicas), "-n", defaultOr(o.Namespace)}
	return run(ctx, runOptions{args: args, workDir: o.WorkDir, env: o.Env})
}
