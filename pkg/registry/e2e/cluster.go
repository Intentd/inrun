package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/tools/cluster"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// preserveKubectlContext captures the active context and returns a function
// that restores it. Returns nil if the current context cannot be read.
func preserveKubectlContext() func() {
	origCtx, err := currentKubectlContext()
	if err != nil || origCtx == "" {
		return nil
	}

	return func() {
		if out, err := exec.Command("kubectl", "config", "use-context", origCtx).CombinedOutput(); err != nil {
			fmt.Printf("  ! Could not restore kubectl context to %q: %v\n%s\n", origCtx, err, out)
		} else {
			fmt.Printf("\n→ kubectl context restored to %q\n", origCtx)
		}
	}
}

// buildClient constructs the Go Kubernetes client and REST config from the active
// kubeconfig after the cluster context has been established by ensureCluster.
func (r *Runner) buildClient() error {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return err
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	r.cfg = cfg
	r.cs = cs
	return nil
}

// requiresCluster checks if cluster is required for this run
func (r *Runner) requiresCluster() (bool, error) {
	provider := r.provider()

	switch provider {
	case types.ProviderNone:
		fmt.Printf("→ No cluster (provider: none) — running local expectations only\n")
		return false, nil
	case types.ProviderKind:
		return true, nil
	default:
		return false, fmt.Errorf("provider %q not supported", provider)
	}
}

// ensureCluster sets up the cluster according to the spec.
func (r *Runner) ensureCluster(ctx context.Context) error {
	if r.useCurrentCtx {
		fmt.Printf("→ Using current cluster context...\n")
		return nil
	}

	if r.clusterCtx != "" {
		fmt.Printf("→ Using existing cluster context %q...\n", r.clusterCtx)
		if out, err := exec.CommandContext(ctx, "kubectl", "config", "use-context", r.clusterCtx).CombinedOutput(); err != nil {
			return fmt.Errorf("switching context: %w\n%s", err, out)
		}
		fmt.Printf("  %s Using context %s\n", successMark(), r.clusterCtx)
		return nil
	}

	name := r.clusterName()
	spec := r.e2e.Spec.Cluster

	if !spec.Reuse {
		// Delete if already exists for a clean state.
		if clusterExists(name) {
			fmt.Printf("→ Recreating cluster '%s' (reuse: false)...\n", name)
			if err := deleteKindCluster(ctx, name); err != nil {
				return fmt.Errorf("deleting old cluster: %w", err)
			}
		}
	}

	return cluster.EnsureKindCluster(name, r.workers, r.kindVersion)
}

func (r *Runner) deleteCluster(ctx context.Context) error {
	return deleteKindCluster(ctx, r.clusterName())
}

func (r *Runner) clusterName() string {
	if r.e2e.Spec.Cluster.Name != "" {
		return r.e2e.Spec.Cluster.Name
	}
	return defaultClusterName
}

func (r *Runner) provider() types.ClusterProvider {
	return r.e2e.Spec.Cluster.EffectiveProvider()
}

func clusterExists(name string) bool {
	out, _ := exec.Command("kind", "get", "clusters").Output()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

func deleteKindCluster(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
