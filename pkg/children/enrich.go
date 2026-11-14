package children

import (
	"context"

	"github.com/intentd/inrun/pkg/types"
	kubechildren "github.com/intentd/tools/children"
	"github.com/intentd/tools/kubeclient"
)

// Each enricher runs the tools/children enrichment only when the CRD turns it
// on. The gate stays here: which identifiers turn which enrichment on is
// Inrun's vocabulary (enrichmentEnabled), and ReadChildren changes the CRD's
// enabled targets between its two phases.

func enrichGroupWithCronJobChildren(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("cronjob", crd) {
		kubechildren.EnrichGroupWithCronJobChildren(ctx, kube, m)
	}
}

func enrichGroupWithEndpoints(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("endpoints", crd) {
		kubechildren.EnrichGroupWithEndpoints(ctx, kube, m)
	}
}

func enrichGroupWithIngressData(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("ingress", crd) {
		kubechildren.EnrichGroupWithIngressData(ctx, kube, m)
	}
}

func enrichGroupWithNode(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("node", crd) {
		kubechildren.EnrichGroupWithNode(ctx, kube, m)
	}
}

func enrichGroupWithPods(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry, ownerKind string) {
	if enrichmentEnabled("pods", crd) {
		kubechildren.EnrichGroupWithPods(ctx, kube, m, ownerKind)
	}
}

func enrichGroupWithPVC(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("pv", crd) {
		kubechildren.EnrichGroupWithPVC(ctx, kube, m)
	}
}

func enrichGroupWithPV(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("pv", crd) {
		kubechildren.EnrichGroupWithPV(ctx, kube, m)
	}
}

func enrichGroupWithReplicaSets(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if crd.ShouldEnrich("replicasets") {
		kubechildren.EnrichGroupWithReplicaSets(ctx, kube, m)
	}
}

func enrichGroupWithBackingPods(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("backingpods", crd) {
		kubechildren.EnrichGroupWithBackingPods(ctx, kube, m)
	}
}

func enrichGroupWithStatefulSetPVCs(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("pvcs", crd) {
		kubechildren.EnrichGroupWithStatefulSetPVCs(ctx, kube, m)
	}
}

func enrichGroupWithStorageClass(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if enrichmentEnabled("storageclass", crd) {
		kubechildren.EnrichGroupWithStorageClass(ctx, kube, m)
	}
}

func enrichGroupWithWarnings(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry, kind string) {
	if enrichmentEnabled("events", crd) {
		kubechildren.EnrichGroupWithWarnings(ctx, kube, m, kind)
	}
}
