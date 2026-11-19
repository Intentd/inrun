package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/intentd/inrun/pkg/registry/e2e/kubectl"
	"github.com/intentd/inrun/pkg/tools/inruncluster"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/tools/cluster"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// preserveKubectlContext captures the active context and returns a function
// that restores it. Returns nil if the current context cannot be read.
func preserveKubectlContext(ctx context.Context) func() {
	origCtx, err := kubectl.CurrentContext(ctx)
	if err != nil || origCtx == "" {
		return nil
	}

	return func() {
		if out, err := kubectl.UseContext(ctx, origCtx); err != nil {
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

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	r.cfg = cfg
	r.cs = cs
	r.dyn = dyn
	r.mapper = restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
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
		if out, err := kubectl.UseContext(ctx, r.clusterCtx); err != nil {
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

	return cluster.EnsureKindCluster(name, inruncluster.Kind(r.workers, r.kindVersion))
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

// kindClusterName returns the kind cluster's name. When the runner created
// the cluster, it has the name directly; when it is on --use-current or
// --cluster, the context must be kind-<name>.
func (r *Runner) kindClusterName(ctx context.Context) (string, bool) {
	clusterCtx := r.effectiveContext(ctx)
	const prefix = "kind-"
	if !strings.HasPrefix(clusterCtx, prefix) {
		return "", false
	}
	return strings.TrimPrefix(clusterCtx, prefix), true
}

// effectiveContext returns the kubectl context the run is on:
//   - the one selected with --cluster,
//   - the kind context the run created (kind-<name>) when it provisions one,
//   - otherwise the current context — --use-current, or no cluster at all.
//
// It is the single source of truth for "which cluster is this run against,"
// so kindClusterName and the image loader agree with ensureCluster.
func (r *Runner) effectiveContext(ctx context.Context) string {
	if r.clusterCtx != "" {
		return r.clusterCtx
	}
	// A kind cluster the run creates is selected on kind-<name>; --use-current
	// says to stay on whatever context is already active.
	if r.needsCluster && !r.useCurrentCtx {
		return "kind-" + r.clusterName()
	}
	if ctx, err := kubectl.CurrentContext(ctx); err == nil {
		return ctx
	}
	return ""
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
