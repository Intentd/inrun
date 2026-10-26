package kubectl

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func kubectlDelete(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlDelete, env types.EnvResolver, workDir string) error {
	var args []string
	if e.LeaderElection != nil {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		pod, leaseNs, err := ResolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return fmt.Errorf("kubectl delete: %w", err)
		}
		args = []string{"delete", "pod", pod, "-n", leaseNs}
	} else if e.File != "" {
		file := e.File
		if !filepath.IsAbs(file) && workDir != "" {
			file = filepath.Join(workDir, file)
		}
		args = []string{"delete", "-f", file}
	} else {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		args = []string{"delete", e.Kind, e.Name, "-n", ns}
	}
	if e.IgnoreNotFound {
		args = append(args, "--ignore-not-found")
	}
	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		ref := e.File
		if ref == "" {
			ref = e.Kind + "/" + e.Name
		}
		return fmt.Errorf("kubectl delete %s: %s", ref, out)
	}
	return nil
}

func kubectlApply(ctx context.Context, e types.E2EKubectlApply, env types.EnvResolver, workDir string) error {
	var label string
	var out []byte
	var runErr error

	if e.Inline != "" {
		label = "kubectl apply (inline)"
		args := []string{"apply", "-f", "-"}
		if e.Namespace != "" {
			args = append(args, "-n", e.Namespace)
		}
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		if workDir != "" {
			cmd.Dir = workDir
		}
		cmd.Stdin = strings.NewReader(e.Inline)
		out, runErr = cmd.CombinedOutput()
	} else {
		file := e.File
		if !filepath.IsAbs(file) && workDir != "" {
			file = filepath.Join(workDir, file)
		}
		label = fmt.Sprintf("kubectl apply -f %s", e.File)
		args := []string{"apply", "-f", file}
		if e.Namespace != "" {
			args = append(args, "-n", e.Namespace)
		}
		var s string
		s, runErr = Run(ctx, workDir, env, args...)
		out = []byte(s)
	}

	return assertKubectlApplyOutput(label, out, runErr, e)
}

// assertKubectlApplyOutput checks the captured exit code and output of a
// kubectl apply run against e's expectations. Split out from
// kubectlApply so the exit-code/assertion logic is testable without
// actually shelling out to kubectl — runErr is whatever
// exec.Cmd.CombinedOutput() (or an equivalent) returned.
func assertKubectlApplyOutput(label string, out []byte, runErr error, e types.E2EKubectlApply) error {
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("%s: %w", label, runErr)
		}
	}

	if exitCode != e.ExitCode {
		return fmt.Errorf("%s: expected exit code %d, got %d\noutput: %s",
			label, e.ExitCode, exitCode, strings.TrimSpace(string(out)))
	}
	if err := e.E2EAssertions.Check(string(out)); err != nil {
		return fmt.Errorf("%s: %w\noutput: %s", label, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func kubectlPatch(ctx context.Context, e types.E2EKubectlPatch, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}
	patchType := e.Type
	if patchType == "" {
		patchType = "merge"
	}
	args := []string{"patch", e.Kind, e.Name, "-n", ns, "--type", patchType, "-p", e.Patch}
	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return fmt.Errorf("kubectl patch %s/%s: %s", e.Kind, e.Name, out)
	}
	return nil
}

func kubectlExec(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlExec, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	var pod string
	if e.LeaderElection != nil {
		var leaseNs string
		var err error
		pod, leaseNs, err = ResolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return fmt.Errorf("kubectl exec: %w", err)
		}
		ns = leaseNs
	} else if e.LabelSelector != "" {
		var err error
		pod, err = Run(ctx, workDir, env,
			"get", "pod", "-n", ns, "-l", e.LabelSelector,
			"-o", "jsonpath={.items[0].metadata.name}")
		if err != nil || strings.TrimSpace(pod) == "" {
			return fmt.Errorf("kubectl exec: no pod found for selector %q in %s", e.LabelSelector, ns)
		}
		pod = strings.TrimSpace(pod)
	} else {
		pod = e.Name
	}

	args := []string{"exec", "-n", ns, pod}
	if e.Container != "" {
		args = append(args, "-c", e.Container)
	}
	args = append(args, "--")
	args = append(args, e.Command...)

	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return fmt.Errorf("kubectl exec %s: %s", pod, out)
	}

	out, err = applyExtract(ctx, types.ExtractRequest{
		WorkDir:   workDir,
		Output:    out,
		Extractor: types.Extractor{JSONPath: e.JSONPath, JQ: e.JQ, YQ: e.YQ},
	})
	if err != nil {
		return fmt.Errorf("kubectl exec %s: %w", pod, err)
	}

	if err := e.JSON.Check(out); err != nil {
		return fmt.Errorf("kubectl exec %s: %w\noutput: %s", pod, err, out)
	}
	return assertKubectlExecOutput(pod, out, e)
}

// assertKubectlExecOutput applies e's assertions to already-captured (and
// json/jq/yq-extracted) exec output. Split out so this logic is testable
// without kubectl or a cluster.
func assertKubectlExecOutput(pod, out string, e types.E2EKubectlExec) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl exec %s: %w\noutput: %s", pod, err, out)
	}
	return nil
}
