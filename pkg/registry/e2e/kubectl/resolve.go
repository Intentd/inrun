package kubectl

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ExecTarget is the resolved pod and namespace an exec runs against.
type ExecTarget struct {
	Pod       string
	Namespace string
}

// resolveExecTarget resolves LeaderElection, LabelSelector (via a pod Get), or
// Name into the pod Exec runs against: the pre-step of readExec, in a step or
// a post block, so leader-election and selector resolution live in one place.
func resolveExecTarget(
	ctx context.Context,
	cs kubernetes.Interface,
	e types.E2EKubectlExec,
	env types.EnvResolver,
	workDir string,
) (ExecTarget, error) {
	ns := defaultOr(e.Namespace)

	if e.LeaderElection != nil {
		pod, leaseNs, err := resolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return ExecTarget{}, fmt.Errorf("kubectl exec: %w", err)
		}
		return ExecTarget{Pod: pod, Namespace: leaseNs}, nil
	}

	if e.LabelSelector != "" {
		pod, err := Get(ctx, GetOptions{
			Kind:          "pod",
			LabelSelector: e.LabelSelector,
			Namespace:     ns,
			Format:        "jsonpath={.items[0].metadata.name}",
			WorkDir:       workDir,
			Env:           env,
		})
		if err != nil || strings.TrimSpace(pod) == "" {
			return ExecTarget{}, fmt.Errorf("kubectl exec: no pod found for selector %q in %s", e.LabelSelector, ns)
		}
		return ExecTarget{Pod: strings.TrimSpace(pod), Namespace: ns}, nil
	}

	return ExecTarget{Pod: e.Name, Namespace: ns}, nil
}

// LogsTarget is the resolved target for a logs invocation: a pod name, or a
// label selector, plus the namespace the pod lives in.
type LogsTarget struct {
	Pod           string
	LabelSelector string
	Namespace     string
}

// resolveLogsTarget resolves LeaderElection or LabelSelector or Name into the
// target Logs runs against. Empty Namespace means "default". It is the
// pre-step of readLogs, in a step or a post block.
func resolveLogsTarget(
	ctx context.Context,
	cs kubernetes.Interface,
	e types.E2EKubectlLogs,
) (LogsTarget, error) {
	ns := defaultOr(e.Namespace)
	if e.LeaderElection != nil {
		pod, leaseNs, err := resolveLeaderHolder(ctx, cs, e.LeaderElection, ns)
		if err != nil {
			return LogsTarget{}, fmt.Errorf("kubectl logs: %w", err)
		}
		return LogsTarget{Pod: pod, Namespace: leaseNs}, nil
	}
	if e.LabelSelector != "" {
		return LogsTarget{LabelSelector: e.LabelSelector, Namespace: ns}, nil
	}
	return LogsTarget{Pod: e.Name, Namespace: ns}, nil
}

// resolveLeaderHolder looks up the holder of a Kubernetes Lease and returns the
// pod name and the namespace the lease lives in. leaseNs defaults to ns when
// the LeaderElection struct leaves it empty.
func resolveLeaderHolder(ctx context.Context, cs kubernetes.Interface, le *types.E2EKubectlLeaderElection, ns string) (pod, leaseNs string, err error) {
	leaseNs = le.Namespace
	if leaseNs == "" {
		leaseNs = ns
	}
	lease, err := cs.CoordinationV1().Leases(leaseNs).Get(ctx, le.Lease, metav1.GetOptions{})
	if err != nil {
		return "", leaseNs, fmt.Errorf("lease %s/%s: %w", leaseNs, le.Lease, err)
	}
	if lease.Spec.HolderIdentity == nil || strings.TrimSpace(*lease.Spec.HolderIdentity) == "" {
		return "", leaseNs, fmt.Errorf("lease %s/%s has no holder yet", leaseNs, le.Lease)
	}
	return strings.TrimSpace(*lease.Spec.HolderIdentity), leaseNs, nil
}
