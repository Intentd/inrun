package children

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithBackingPods lists the pods that match each Service's selector
// and embeds pod summaries under "_backingPods". A no-op when backingpods
// enrichment is not enabled on the CRD.
//
// Unlike Deployment/StatefulSet, Service uses spec.selector (a flat label map),
// not spec.selector.matchLabels. Headless and ExternalName services with no
// selector are skipped.
func enrichGroupWithBackingPods(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("backingpods", crd) {
		return
	}

	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		meta, _ := obj["metadata"].(map[string]interface{})
		if meta == nil {
			continue
		}
		ns, _ := meta["namespace"].(string)
		if ns == "" {
			continue
		}
		selector := servicePodSelector(obj)
		if selector == "" {
			continue
		}
		list, err := kube.DynamicClient().
			Resource(PodGVR).
			Namespace(ns).
			List(ctx, metav1.ListOptions{
				LabelSelector:   selector,
				ResourceVersion: "0",
			})
		if err != nil || list == nil {
			continue
		}
		pods := make([]interface{}, 0, len(list.Items))
		for i := range list.Items {
			pods = append(pods, trimPod(list.Items[i].Object))
		}
		if len(pods) > 0 {
			obj["_backingPods"] = pods
		}
	}
}

// servicePodSelector builds a comma-separated label selector from spec.selector,
// which on a Service is a flat map (not matchLabels). Returns "" when absent.
func servicePodSelector(obj map[string]interface{}) string {
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return ""
	}
	sel, _ := spec["selector"].(map[string]interface{})
	if len(sel) == 0 {
		return ""
	}
	parts := make([]string, 0, len(sel))
	for k, v := range sel {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ",")
}
