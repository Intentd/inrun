package post

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// RunOnFailure runs all onFailure commands and kubectl operations, printing
// their output to the terminal. Never fails — diagnostics must not interrupt
// the teardown path.
func RunOnFailure(ctx context.Context, f *types.E2EOnFailure, workDir string, cs kubernetes.Interface) {
	if f == nil {
		return
	}

	runPostCommands(ctx, "On Failure", f.Commands, f.Kubectl, workDir, cs)
}

// RunFinally runs all finally commands and kubectl operations, printing
// their output to the terminal. Never fails — a broken finally block must not
// mask the test result.
func RunFinally(ctx context.Context, f *types.E2EFinally, workDir string, cs kubernetes.Interface) {
	if f == nil || f.Empty() {
		return
	}

	runPostCommands(ctx, "Finally", f.Commands, f.Kubectl, workDir, cs)
}

// runPostCommands executes post-expectation commands and kubectl operations,
// printing their output to the terminal. Never fails — post-expectation
// operations must not mask the original test result.
func runPostCommands(
	ctx context.Context,
	name string,
	commands []string,
	kubectl *types.E2EKubectl,
	workDir string,
	cs kubernetes.Interface,
) {
	if len(commands) == 0 && kubectl == nil {
		return
	}

	fmt.Printf("\n── %s ──\n", name)

	for i, cmd := range commands {
		cmd := cmd
		printDiag(fmt.Sprintf("commands[%d]: %s", i, cmd), func() (string, error) {
			c := exec.CommandContext(ctx, "sh", "-c", cmd)
			if workDir != "" {
				c.Dir = workDir
			}
			out, err := c.CombinedOutput()
			return strings.TrimSpace(string(out)), err
		})
	}

	if kubectl != nil {
		printOnPostKubectl(ctx, kubectl, workDir, cs)
	}
}

// printOnPostKubectl prints diagnostic output for each supported kubectl subcommand.
// Assertion fields on the DSL structs are ignored — output is always printed.
func printOnPostKubectl(ctx context.Context, k *types.E2EKubectl, workDir string, cs kubernetes.Interface) {
	// kubectl get
	for _, e := range k.Get {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		var args []string
		if e.Field != "" {
			field := e.Field
			if !strings.HasPrefix(field, ".") {
				field = "." + field
			}
			args = []string{"get", e.Kind, e.Name, "-n", ns, "-o", "jsonpath={" + field + "}"}
		} else {
			format := e.Format
			if format == "" {
				format = "yaml"
			}
			args = []string{"get", e.Kind, e.Name, "-n", ns, "-o", format}
		}
		a := args
		printDiag("kubectl "+strings.Join(a, " "), func() (string, error) {
			return runKubectl(ctx, workDir, types.NoopEnvResolver(), a...)
		})
	}

	// kubectl logs
	for _, e := range k.Logs {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		args := []string{"logs", "-n", ns}
		if e.LeaderElection != nil {
			if pod, leaseNs, err := runWithLeader(ctx, cs, e.LeaderElection, ns); err == nil {
				args = []string{"logs", "-n", leaseNs, pod}
			}
		} else if e.LabelSelector != "" {
			args = append(args, "-l", e.LabelSelector)
		} else {
			args = append(args, e.Name)
		}
		if e.Container != "" {
			args = append(args, "-c", e.Container)
		}
		if e.Since != "" {
			args = append(args, "--since="+e.Since)
		}
		a := args
		printDiag("kubectl "+strings.Join(a, " "), func() (string, error) {
			return runKubectl(ctx, workDir, types.NoopEnvResolver(), a...)
		})
	}

	// kubectl describe
	for _, e := range k.Describe {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		args := []string{"describe", e.Kind, "-n", ns}
		if e.Name != "" {
			args = append(args, e.Name)
		} else if e.LabelSelector != "" {
			args = append(args, "-l", e.LabelSelector)
		}
		a := args
		printDiag("kubectl "+strings.Join(a, " "), func() (string, error) {
			return runKubectl(ctx, workDir, types.NoopEnvResolver(), a...)
		})
	}

	// kubectl events
	for _, e := range k.Events {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		args := []string{"events", "-n", ns, "--for=" + e.Kind + "/" + e.Name}
		a := args
		printDiag("kubectl "+strings.Join(a, " "), func() (string, error) {
			return runKubectl(ctx, workDir, types.NoopEnvResolver(), a...)
		})
	}

	// kubectl exec
	for _, e := range k.Exec {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		var podName string
		if e.LeaderElection != nil {
			if pod, leaseNs, err := runWithLeader(ctx, cs, e.LeaderElection, ns); err == nil {
				podName = pod
				ns = leaseNs
			}
		} else if e.LabelSelector != "" {
			if out, err := runKubectl(ctx, workDir, types.NoopEnvResolver(), "get", "pod", "-n", ns, "-l", e.LabelSelector, "-o", "jsonpath={.items[0].metadata.name}"); err == nil {
				podName = strings.TrimSpace(out)
			}
		} else {
			podName = e.Name
		}
		if podName == "" {
			continue
		}
		args := []string{"exec", "-n", ns, podName, "--"}
		args = append(args, e.Command...)
		a := args
		printDiag("kubectl "+strings.Join(a, " "), func() (string, error) {
			return runKubectl(ctx, workDir, types.NoopEnvResolver(), a...)
		})
	}
}

// printDiag runs fn, prints its output indented under label. Never fails.
func printDiag(label string, run func() (string, error)) {
	fmt.Printf("\n  ─ %s\n", label)
	out, err := run()
	if err != nil && strings.TrimSpace(out) == "" {
		fmt.Printf("  ! %v\n", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fmt.Printf("  %s\n", line)
	}
}
