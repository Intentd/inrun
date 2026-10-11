package children

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithWarnings embeds warning events under "_warnings" for every
// resource in the group. A no-op when events enrichment is not enabled on the CRD.
//
// kind is the Kubernetes Kind used to scope the event field selector
// (involvedObject.kind). Pass "" to skip the kind filter — used for custom
// resources whose exact kind is not known statically.
func enrichGroupWithWarnings(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry, kind string) {
	if !enrichmentEnabled("events", crd) {
		return
	}

	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		meta, _ := obj["metadata"].(map[string]interface{})
		ns, name := "", ""
		if meta != nil {
			ns, _ = meta["namespace"].(string)
			name, _ = meta["name"].(string)
		}
		if name == "" {
			continue
		}
		enrichWithWarnings(ctx, kube, ns, name, kind, obj)
	}
}

// enrichWithWarnings fetches Warning events scoped to the named resource and
// embeds a list of {reason, message, count, lastTimestamp} maps under _warnings.
//
// Field selector: involvedObject.name=<name>,type=Warning
// When kind is non-empty: involvedObject.kind=<kind> is appended.
// ResourceVersion "0" serves from the informer cache — no quorum read.
//
// For workload kinds (Deployment, StatefulSet, ReplicaSet) pod-level Warning
// events are also aggregated, because container failures (ImagePullBackOff,
// OOMKilled, etc.) are recorded on the Pod, not on the workload itself.
func enrichWithWarnings(ctx context.Context, kube kubeclient.Interface, ns, name, kind string, obj map[string]interface{}) {
	if ns == "" {
		return
	}
	selector := fmt.Sprintf("involvedObject.name=%s,type=Warning", name)
	if kind != "" {
		selector += fmt.Sprintf(",involvedObject.kind=%s", kind)
	}
	var warnings []interface{}
	list, err := kube.DynamicClient().
		Resource(EventGVR).
		Namespace(ns).
		List(ctx, metav1.ListOptions{
			FieldSelector:   selector,
			ResourceVersion: "0",
		})
	if err == nil && list != nil {
		for i := range list.Items {
			warnings = append(warnings, buildWarningSummary(list.Items[i].Object))
		}
	}

	// Pod-level events for workloads — container failures are reported on the Pod.
	if kind == "Deployment" || kind == "StatefulSet" || kind == "ReplicaSet" {
		warnings = append(warnings, fetchOwnedPodWarnings(ctx, kube, ns, obj)...)
	}

	if len(warnings) > 0 {
		obj["_warnings"] = warnings
	}
}

// fetchOwnedPodWarnings lists pods matching the workload's label selector and
// returns aggregated Warning events from each pod. One API call per pod, all
// served from the informer cache (ResourceVersion "0").
func fetchOwnedPodWarnings(ctx context.Context, kube kubeclient.Interface, ns string, obj map[string]interface{}) []interface{} {
	selector := podLabelSelector(obj)
	if selector == "" {
		return nil
	}
	podList, err := kube.DynamicClient().
		Resource(PodGVR).
		Namespace(ns).
		List(ctx, metav1.ListOptions{
			LabelSelector:   selector,
			ResourceVersion: "0",
		})
	if err != nil || podList == nil {
		return nil
	}
	var warnings []interface{}
	for i := range podList.Items {
		meta, _ := podList.Items[i].Object["metadata"].(map[string]interface{})
		podName, _ := meta["name"].(string)
		if podName == "" {
			continue
		}
		evList, err := kube.DynamicClient().
			Resource(EventGVR).
			Namespace(ns).
			List(ctx, metav1.ListOptions{
				FieldSelector:   fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod,type=Warning", podName),
				ResourceVersion: "0",
			})
		if err != nil || evList == nil {
			continue
		}
		for j := range evList.Items {
			warnings = append(warnings, buildWarningSummary(evList.Items[j].Object))
		}
	}
	return warnings
}

// buildWarningSummary extracts the fields note functions navigate from _warnings.
func buildWarningSummary(obj map[string]interface{}) map[string]interface{} {
	reason, _ := obj["reason"].(string)
	message, _ := obj["message"].(string)
	lastTimestamp, _ := obj["lastTimestamp"].(string)
	count := toInt64(obj["count"])
	return map[string]interface{}{
		"reason":        reason,
		"message":       message,
		"count":         count,
		"lastTimestamp": lastTimestamp,
	}
}
