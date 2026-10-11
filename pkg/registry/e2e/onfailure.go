package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// runOnFailure runs all onFailure commands and kubectl operations, printing
// their output to the terminal. Never fails — diagnostics must not interrupt
// the teardown path.
func runOnFailure(ctx context.Context, f *types.E2EOnFailure, workDir string, cs kubernetes.Interface) {
	if f == nil {
		return
	}
	fmt.Printf("\n── On Failure ──\n")

	for i, cmd := range f.Commands {
		printDiag(fmt.Sprintf("commands[%d]: %s", i, cmd), func() (string, error) {
			out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
			return strings.TrimSpace(string(out)), err
		})
	}

	if f.Kubectl != nil {
		printOnFailureKubectl(ctx, f.Kubectl, workDir, cs)
	}
}

// printOnFailureKubectl prints diagnostic output for each supported kubectl subcommand.
// Assertion fields on the DSL structs are ignored — output is always printed.
func printOnFailureKubectl(ctx context.Context, k *types.E2EKubectl, workDir string, cs kubernetes.Interface) {
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
			return runKubectl(ctx, workDir, a...)
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
			if pod, leaseNs, err := resolveLeaderHolder(ctx, cs, e.LeaderElection, ns); err == nil {
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
			return runKubectl(ctx, workDir, a...)
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
			return runKubectl(ctx, workDir, a...)
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
			return runKubectl(ctx, workDir, a...)
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
			if pod, leaseNs, err := resolveLeaderHolder(ctx, cs, e.LeaderElection, ns); err == nil {
				podName = pod
				ns = leaseNs
			}
		} else if e.LabelSelector != "" {
			if out, err := runKubectl(ctx, workDir, "get", "pod", "-n", ns, "-l", e.LabelSelector, "-o", "jsonpath={.items[0].metadata.name}"); err == nil {
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
			return runKubectl(ctx, workDir, a...)
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
