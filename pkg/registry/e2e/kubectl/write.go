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

// The write verbs: each runs one kubectl entry that changes the cluster and
// returns kubectl's output with its error. Write runs them before the
// assertions, once per attempt; Post runs them once.

func writeApply(ctx context.Context, e types.E2EKubectlApply, env types.EnvResolver, workDir string) (string, error) {
	var label string
	var out []byte
	var runErr error

	if e.Inline != "" {
		label = "kubectl apply (inline)"
		s, err := ApplyInline(ctx, ApplyInlineOptions{
			Content:   e.Inline,
			Namespace: e.Namespace,
			WorkDir:   workDir,
			Env:       env,
		})
		out, runErr = []byte(s), err
	} else {
		file := e.File
		if !filepath.IsAbs(file) && workDir != "" {
			file = filepath.Join(workDir, file)
		}
		label = fmt.Sprintf("kubectl apply -f %s", e.File)
		s, err := Apply(ctx, ApplyOptions{
			File:      file,
			Namespace: e.Namespace,
			WorkDir:   workDir,
			Env:       env,
		})
		out, runErr = []byte(s), err
	}

	return string(out), assertApplyOutput(label, out, runErr, e)
}

// assertApplyOutput checks the captured exit code and output of a
// kubectl apply run against e's expectations. Split out from
// writeApply so the exit-code/assertion logic is testable without
// actually shelling out to kubectl — runErr is whatever
// exec.Cmd.CombinedOutput() (or an equivalent) returned.
func assertApplyOutput(label string, out []byte, runErr error, e types.E2EKubectlApply) error {
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

	return assertOutput(string(out), e.TextAssertion, e.Assert, label)
}

func writePatch(ctx context.Context, e types.E2EKubectlPatch, env types.EnvResolver, workDir string) (string, error) {
	if e.File != "" {
		out, err := PatchFile(ctx, PatchFileOptions{
			File:      e.File,
			Type:      e.Type,
			Patch:     e.Patch,
			PatchFile: e.PatchFile,
			WorkDir:   workDir,
			Env:       env,
		})
		if err != nil {
			return out, fmt.Errorf("kubectl patch -f %s: %s", e.File, out)
		}
		return out, nil
	}
	out, err := Patch(ctx, PatchOptions{
		Kind:      e.Kind,
		Name:      e.Name,
		Namespace: e.Namespace,
		Type:      e.Type,
		Patch:     e.Patch,
		PatchFile: e.PatchFile,
		WorkDir:   workDir,
		Env:       env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl patch %s/%s: %s", e.Kind, e.Name, out)
	}
	return out, nil
}

func writeDelete(ctx context.Context, cs kubernetes.Interface, e types.E2EKubectlDelete, env types.EnvResolver, workDir string) (string, error) {
	o := DeleteOptions{
		IgnoreNotFound: e.IgnoreNotFound,
		WorkDir:        workDir,
		Env:            env,
	}

	switch {
	case e.LeaderElection != nil:
		ns := defaultOr(e.Namespace)
		pod, leaseNs, err := resolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return "", fmt.Errorf("kubectl delete: %w", err)
		}
		o.Kind, o.Name, o.Namespace = "pod", pod, leaseNs

	case e.File != "":
		file := e.File
		if !filepath.IsAbs(file) && workDir != "" {
			file = filepath.Join(workDir, file)
		}
		o.File = file

	default:
		o.Kind, o.Name, o.Namespace = e.Kind, e.Name, e.Namespace
	}

	out, err := Delete(ctx, o)
	if err != nil {
		ref := e.File
		if ref == "" {
			ref = e.Kind + "/" + e.Name
		}
		return out, fmt.Errorf("kubectl delete %s: %s", ref, out)
	}
	return out, nil
}

func writeRestart(ctx context.Context, e types.E2EKubectlRestart, env types.EnvResolver, workDir string) (string, error) {
	target := e.Kind + "/" + e.Name
	out, err := Rollout(ctx, RolloutOptions{
		Sub:       RolloutRestart,
		Target:    target,
		Namespace: e.Namespace,
		WorkDir:   workDir,
		Env:       env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl rollout restart %s: %w", target, err)
	}
	if e.Ready == nil || *e.Ready {
		status, err := Rollout(ctx, RolloutOptions{
			Sub:       RolloutStatus,
			Target:    target,
			Namespace: e.Namespace,
			WorkDir:   workDir,
			Env:       env,
		})
		out += status
		if err != nil {
			return out, fmt.Errorf("kubectl rollout status %s: %s", target, status)
		}
	}
	return out, nil
}

func writeScale(ctx context.Context, e types.E2EKubectlScale, env types.EnvResolver, workDir string) (string, error) {
	target := e.Kind + "/" + e.Name
	out, err := Scale(ctx, ScaleOptions{
		Target:    target,
		Replicas:  e.Replicas,
		Namespace: e.Namespace,
		WorkDir:   workDir,
		Env:       env,
	})
	if err != nil {
		return out, fmt.Errorf("kubectl scale %s: %w", target, err)
	}
	if e.Ready == nil || *e.Ready {
		status, err := Rollout(ctx, RolloutOptions{
			Sub:       RolloutStatus,
			Target:    target,
			Namespace: e.Namespace,
			WorkDir:   workDir,
			Env:       env,
		})
		out += status
		if err != nil {
			return out, fmt.Errorf("kubectl rollout status %s: %s", target, status)
		}
	}
	return out, nil
}
