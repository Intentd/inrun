package kubectl

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Check runs the block's assertions (get, logs, describe, …).
func Check(ctx context.Context, k *types.E2EKubectl, env types.EnvResolver, workDir string, cs kubernetes.Interface, cfg *rest.Config) error {
	for i, e := range k.Get {
		if err := kubectlGet(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.get[%d]: %w", i, err)
		}
	}
	for i, e := range k.Logs {
		if err := kubectlLogs(ctx, cs, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.logs[%d]: %w", i, err)
		}
	}
	for i, e := range k.Describe {
		if err := kubectlDescribe(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.describe[%d]: %w", i, err)
		}
	}
	for i, e := range k.Exec {
		if err := kubectlExec(ctx, cs, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.exec[%d]: %w", i, err)
		}
	}
	for i, e := range k.PortForward {
		if err := kubectlPortForward(ctx, cs, cfg, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.port-forward[%d]: %w", i, err)
		}
	}
	for i, e := range k.Events {
		if err := kubectlEvents(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.events[%d]: %w", i, err)
		}
	}
	for i, e := range k.Auth {
		if err := kubectlAuth(ctx, cs, e); err != nil {
			return fmt.Errorf("kubectl.auth[%d]: %w", i, err)
		}
	}
	for i, e := range k.Cp {
		if err := kubectlCp(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.cp[%d]: %w", i, err)
		}
	}
	for i, e := range k.Top {
		if err := kubectlTop(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.top[%d]: %w", i, err)
		}
	}
	return nil
}

// Run executes kubectl in workDir with the expectation's environment and
// returns its combined output. Every kubectl invocation in an expectation
// goes through here, so env declared in spec.env or expect.env reaches the
// command the same way it reaches sh -c in commands:.
func Run(ctx context.Context, workDir string, env types.EnvResolver, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = append(os.Environ(), env.Pairs()...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// CurrentContext returns the current cluster context
func CurrentContext() (string, error) {
	out, err := exec.Command("kubectl", "config", "current-context").Output()
	return strings.TrimSpace(string(out)), err
}

// Mutate runs the block's actions in order: apply → patch →
// restart → scale → delete. verifyExpectation runs them once, before the
// assertions in kubectl.
func Mutate(ctx context.Context, k *types.E2EKubectl, workDir string, env types.EnvResolver, cs kubernetes.Interface) error {
	for i, e := range k.Apply {
		if err := kubectlApply(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.apply[%d]: %w", i, err)
		}
	}
	for i, e := range k.Patch {
		if err := kubectlPatch(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.patch[%d]: %w", i, err)
		}
	}
	for i, e := range k.Restart {
		if err := kubectlRestart(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.restart[%d]: %w", i, err)
		}
	}
	for i, e := range k.Scale {
		if err := kubectlScale(ctx, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.scale[%d]: %w", i, err)
		}
	}
	for i, e := range k.Delete {
		if err := kubectlDelete(ctx, cs, e, env, workDir); err != nil {
			return fmt.Errorf("kubectl.delete[%d]: %w", i, err)
		}
	}
	return nil
}
