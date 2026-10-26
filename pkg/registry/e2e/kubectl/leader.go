package kubectl

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResolveLeaderHolder looks up the holder of a Kubernetes Lease and returns the
// pod name and the namespace the lease lives in. leaseNs defaults to ns when
// the LeaderElection struct leaves it empty.
func ResolveLeaderHolder(ctx context.Context, cs kubernetes.Interface, le *types.E2EKubectlLeaderElection, ns string) (pod, leaseNs string, err error) {
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
