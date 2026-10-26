package api

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
)

// ClusterCheckResult holds the outcome for a single gateway cluster.
type ClusterCheckResult struct {
	Name     string
	Endpoint string
	// CredErr is non-nil when the credential Secret could not be read.
	CredErr error
	// ConnErr is non-nil when the remote API server could not be reached.
	ConnErr error
	// CRDs maps CRD name → error (nil = installed, non-nil = missing or unreachable).
	CRDs map[string]error
}

// Failed reports whether any check step failed for this cluster.
func (r ClusterCheckResult) Failed() bool {
	if r.CredErr != nil || r.ConnErr != nil {
		return true
	}
	for _, err := range r.CRDs {
		if err != nil {
			return true
		}
	}
	return false
}

// CheckClusters connects to each entry in clusters, reads its credential from
// localKube, pings the remote API server, and checks that the CRDs routed to
// that cluster are installed. ownNS is the fallback namespace for secret
// lookups. Results are returned in the same order as sortedClusterKeys would
// produce — callers may range over them directly.
func CheckClusters(
	ctx context.Context,
	bp *blueprint.Blueprint,
	clusters map[string]types.GatewayClusterConfig,
	localKube kubeclient.Interface,
	ownNS string,
) []ClusterCheckResult {
	scheme, _ := bp.Scheme()

	results := make([]ClusterCheckResult, 0, len(clusters))

	for name, cfg := range clusters {
		r := ClusterCheckResult{
			Name:     name,
			Endpoint: cfg.EndpointURL(),
			CRDs:     map[string]error{},
		}

		rc, err := BuildClusterRestConfig(ctx, cfg, localKube, ownNS)
		if err != nil {
			r.CredErr = err
			results = append(results, r)
			continue
		}

		remoteKube, err := kubeclient.NewFromConfig(ctx, rc, scheme)
		if err != nil {
			r.ConnErr = fmt.Errorf("building client: %w", err)
			results = append(results, r)
			continue
		}

		if _, err := remoteKube.Clientset().Discovery().ServerVersion(); err != nil {
			r.ConnErr = fmt.Errorf("unreachable: %w", err)
			results = append(results, r)
			continue
		}

		if bp != nil {
			for _, crd := range bp.ServeEnabledCRDsForCluster(name) {
				_, listErr := remoteKube.DynamicClient().Resource(crd.GVR()).
					List(ctx, metav1.ListOptions{Limit: 1})
				r.CRDs[crd.APITypes.Kind] = listErr
			}
		}

		results = append(results, r)
	}

	return results
}
