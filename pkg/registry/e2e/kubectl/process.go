package kubectl

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

func kubectlCp(ctx context.Context, e types.E2EKubectlCp, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}

	pod := e.Name
	if pod == "" && e.LabelSelector != "" {
		var err error
		pod, err = Run(ctx, workDir, env,
			"get", "pod", "-n", ns, "-l", e.LabelSelector,
			"-o", "jsonpath={.items[0].metadata.name}")
		if err != nil || strings.TrimSpace(pod) == "" {
			return fmt.Errorf("kubectl cp: no pod found for selector %q in %s", e.LabelSelector, ns)
		}
	}

	tmp, err := os.CreateTemp("", "inrun-e2e-cp-*")
	if err != nil {
		return fmt.Errorf("kubectl cp: creating temp file: %w", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	src := fmt.Sprintf("%s/%s:%s", ns, pod, e.Src)
	args := []string{"cp", src, tmp.Name()}
	if e.Container != "" {
		args = append(args, "-c", e.Container)
	}
	if out, err := Run(ctx, workDir, env, args...); err != nil {
		return fmt.Errorf("kubectl cp %s: %s", src, out)
	}

	raw, err := readLocal(tmp.Name())
	if err != nil {
		return fmt.Errorf("kubectl cp: reading temp file: %w", err)
	}
	out := string(raw)

	out, err = applyExtract(ctx, types.ExtractRequest{
		WorkDir: workDir,
		Output:  out,
		Extractor: types.Extractor{
			JSONPath: e.JSONPath,
			JQ:       e.JQ,
			YQ:       e.YQ,
		}})
	if err != nil {
		return fmt.Errorf("kubectl cp %s: %w", src, err)
	}

	return assertKubectlCpOutput(src, out, e)
}

// assertKubectlCpOutput applies e's assertions to the already-copied (and
// jq/yq-extracted) file content. Split out so this logic is testable
// without kubectl or a cluster.
func assertKubectlCpOutput(src, out string, e types.E2EKubectlCp) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl cp %s: %w\noutput: %s", src, err, out)
	}
	return nil
}

func kubectlRestart(ctx context.Context, e types.E2EKubectlRestart, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}
	target := e.Kind + "/" + e.Name
	if _, err := Run(ctx, workDir, env, "rollout", "restart", target, "-n", ns); err != nil {
		return fmt.Errorf("kubectl rollout restart %s: %w", target, err)
	}
	if e.Ready == nil || *e.Ready {
		if out, err := Run(ctx, workDir, env, "rollout", "status", target, "-n", ns); err != nil {
			return fmt.Errorf("kubectl rollout status %s: %s", target, out)
		}
	}
	return nil
}

func kubectlScale(ctx context.Context, e types.E2EKubectlScale, env types.EnvResolver, workDir string) error {
	ns := e.Namespace
	if ns == "" {
		ns = "default"
	}
	target := e.Kind + "/" + e.Name
	replicas := fmt.Sprintf("--replicas=%d", e.Replicas)
	if _, err := Run(ctx, workDir, env, "scale", target, replicas, "-n", ns); err != nil {
		return fmt.Errorf("kubectl scale %s: %w", target, err)
	}
	if e.Ready == nil || *e.Ready {
		if out, err := Run(ctx, workDir, env, "rollout", "status", target, "-n", ns); err != nil {
			return fmt.Errorf("kubectl rollout status %s: %s", target, out)
		}
	}
	return nil
}

func kubectlTop(ctx context.Context, e types.E2EKubectlTop, env types.EnvResolver, workDir string) error {
	kind := strings.ToLower(e.Kind)
	args := []string{"top", kind}
	if kind == "pod" || kind == "pods" {
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		args = append(args, "-n", ns)
		if e.LabelSelector != "" {
			args = append(args, "-l", e.LabelSelector)
		} else if e.Name != "" {
			args = append(args, e.Name)
		}
		if e.Containers {
			args = append(args, "--containers")
		}
	} else if e.Name != "" {
		args = append(args, e.Name)
	}
	out, err := Run(ctx, workDir, env, args...)
	if err != nil {
		return fmt.Errorf("kubectl top %s: %s", kind, out)
	}
	return assertKubectlTopOutput(kind, out, e)
}

// assertKubectlTopOutput applies e's assertions to already-fetched `kubectl
// top` output. Split out so this logic is testable without kubectl or a
// cluster.
func assertKubectlTopOutput(kind, out string, e types.E2EKubectlTop) error {
	if err := e.E2EAssertions.Check(out); err != nil {
		return fmt.Errorf("kubectl top %s: %w\noutput: %s", kind, err, out)
	}
	return nil
}
