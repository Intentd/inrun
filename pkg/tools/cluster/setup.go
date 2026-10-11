package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// HelmInstall installs or upgrades a Helm chart as declared in SetupHelmInstall.
// Uses helm upgrade --install so the call is idempotent.
func HelmInstall(ctx context.Context, h types.SetupHelmInstall) error {
	if err := h.Validate(); err != nil {
		return err
	}

	release := h.ReleaseName()
	namespace := h.EffectiveNamespace()

	chartRef := h.Chart
	if !h.IsLocalChart() {
		repoName := release
		_, _ = exec.CommandContext(ctx, "helm", "repo", "add", repoName, h.Repo).Output()
		_, _ = exec.CommandContext(ctx, "helm", "repo", "update", repoName).Output()
		chartRef = fmt.Sprintf("%s/%s", repoName, h.Chart)
	}

	args := []string{
		"upgrade", "--install",
		release,
		chartRef,
		"--namespace", namespace,
	}
	if h.CreateNamespace {
		args = append(args, "--create-namespace")
	}
	if h.Version != "" {
		args = append(args, "--version", h.Version)
	}
	for _, f := range h.ValueFiles {
		if f != "" {
			args = append(args, "-f", f)
		}
	}
	for k, v := range h.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%v", k, v))
	}
	args = append(args, "--wait", "--timeout", "5m")

	cmd := exec.CommandContext(ctx, "helm", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helm install %s: %w\n%s", chartRef, err, out)
	}
	return nil
}

// HelmUninstall removes a Helm release installed by HelmInstall.
func HelmUninstall(ctx context.Context, h types.SetupHelmInstall) error {
	release := h.ReleaseName()
	namespace := h.EffectiveNamespace()
	cmd := exec.CommandContext(ctx, "helm", "uninstall", release,
		"--namespace", namespace, "--ignore-not-found")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helm uninstall %s: %w\n%s", release, err, out)
	}
	return nil
}

// WaitForResource polls until the described resource exists (and is ready when
// w.Ready is true). Times out after w.Timeout (default 30s).
func WaitForResource(ctx context.Context, w types.SetupWait) error {
	timeout := 30 * time.Second
	if w.Timeout != "" {
		if d, err := utils.ParseTimeDuration(w.Timeout); err == nil {
			timeout = d
		}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if w.Ready {
			var args []string
			if strings.EqualFold(w.Kind, "Deployment") {
				// Deployments — use rollout status.
				args = []string{"rollout", "status", "deployment/" + w.Name, "--timeout=5s"}
				if w.Namespace != "" {
					args = append(args, "-n", w.Namespace)
				}
			} else {
				args = []string{
					"wait",
					fmt.Sprintf("%s/%s", strings.ToLower(w.Kind), w.Name),
					"--for=condition=Ready",
					"--timeout=5s",
				}
				if w.Namespace != "" {
					args = append(args, "-n", w.Namespace)
				}
			}
			if _, err := exec.CommandContext(ctx, "kubectl", args...).Output(); err == nil {
				return nil
			}
		} else {
			// Just existence check: kubectl get
			args := []string{"get", w.Kind, w.Name, "--ignore-not-found", "-o", "name"}
			if w.Namespace != "" {
				args = append(args, "-n", w.Namespace)
			}
			out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
			if err == nil && strings.TrimSpace(string(out)) != "" {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}

	loc := w.Name
	if w.Namespace != "" {
		loc = w.Namespace + "/" + w.Name
	}
	return fmt.Errorf("timed out after %s waiting for %s %s", timeout, w.Kind, loc)
}
