package children

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithPods attaches the resource's Pods (native, trimmed) under
// "_pods" for every resource
// in the group. A no-op when pods enrichment is not enabled on the CRD.
//
// ownerKind filters pods to only those whose immediate ownerReference matches
// the expected controller kind — e.g. "ReplicaSet" for Deployments, "StatefulSet"
// for StatefulSets. This prevents job pods from appearing in a Deployment's
// pod list when both share the same inrun-owner label selector.
func enrichGroupWithPods(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry, ownerKind string) {
	if !enrichmentEnabled("pods", crd) {
		return
	}

	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		meta, _ := obj["metadata"].(map[string]interface{})
		ns := ""
		if meta != nil {
			ns, _ = meta["namespace"].(string)
		}
		enrichWithPods(ctx, kube, ns, obj, ownerKind)
	}
}

// enrichWithPods lists pods matching the resource's spec.selector.matchLabels,
// filters to those owned by a controller of ownerKind, and attaches them
// (native, trimmed) as _pods.
func enrichWithPods(ctx context.Context, kube kubeclient.Interface, ns string, obj map[string]interface{}, ownerKind string) {
	selector := podLabelSelector(obj)
	if selector == "" {
		return
	}
	list, err := kube.DynamicClient().
		Resource(PodGVR).
		Namespace(ns).
		List(ctx, metav1.ListOptions{
			LabelSelector:   selector,
			ResourceVersion: "0",
		})
	if err != nil || list == nil {
		return
	}
	pods := make([]interface{}, 0, len(list.Items))
	for i := range list.Items {
		if !podOwnedBy(list.Items[i].Object, ownerKind) {
			continue
		}
		pods = append(pods, trimPod(list.Items[i].Object))
	}
	obj["_pods"] = pods
}

// podOwnedBy returns true when any ownerReference on the pod has the given kind.
func podOwnedBy(obj map[string]interface{}, kind string) bool {
	meta, _ := obj["metadata"].(map[string]interface{})
	if meta == nil {
		return false
	}
	ownerRefs, _ := meta["ownerReferences"].([]interface{})
	for _, ref := range ownerRefs {
		r, _ := ref.(map[string]interface{})
		if r == nil {
			continue
		}
		if k, _ := r["kind"].(string); k == kind {
			return true
		}
	}
	return false
}

// podLabelSelector builds a comma-separated label selector from spec.selector.matchLabels.
// Returns "" when the field is absent or empty — no selector means no list.
func podLabelSelector(obj map[string]interface{}) string {
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return ""
	}
	sel, _ := spec["selector"].(map[string]interface{})
	if sel == nil {
		return ""
	}
	matchLabels, _ := sel["matchLabels"].(map[string]interface{})
	if len(matchLabels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(matchLabels))
	for k, v := range matchLabels {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ",")
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}
