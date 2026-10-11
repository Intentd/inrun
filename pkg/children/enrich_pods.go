package children

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithPods embeds pod summaries under "_pods" for every resource
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
// filters to those owned by a controller of ownerKind, and embeds summaries
// as _pods in the resource map.
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
		pods = append(pods, buildPodSummary(list.Items[i].Object))
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

// buildPodSummary extracts the fields note functions navigate from _pods.
func buildPodSummary(obj map[string]interface{}) map[string]interface{} {
	meta, _ := obj["metadata"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	status, _ := obj["status"].(map[string]interface{})

	name := ""
	if meta != nil {
		name, _ = meta["name"].(string)
	}
	nodeName := ""
	if spec != nil {
		nodeName, _ = spec["nodeName"].(string)
	}
	podIP, phase := "", ""
	if status != nil {
		podIP, _ = status["podIP"].(string)
		phase, _ = status["phase"].(string)
	}
	return map[string]interface{}{
		"name":         name,
		"ip":           podIP,
		"phase":        phase,
		"ready":        isPodReady(status),
		"node":         nodeName,
		"restartCount": podTotalRestarts(status),
		"ordinal":      podOrdinal(name),
		"exitCode":     podExitCode(status),
		"containers":   buildContainerSummaries(status),
	}
}

// buildContainerSummaries extracts per-container state from pod status.containerStatuses.
// Each entry: {name, image, state, reason, ready, restartCount}
//
// state is one of: "Running", "Waiting", "Terminated", ""
// reason is state.waiting.reason or state.terminated.reason — e.g. "CrashLoopBackOff"
func buildContainerSummaries(status map[string]interface{}) []interface{} {
	if status == nil {
		return nil
	}
	containerStatuses, _ := status["containerStatuses"].([]interface{})
	if len(containerStatuses) == 0 {
		return nil
	}
	result := make([]interface{}, 0, len(containerStatuses))
	for _, cs := range containerStatuses {
		csMap, _ := cs.(map[string]interface{})
		if csMap == nil {
			continue
		}
		name, _ := csMap["name"].(string)
		image, _ := csMap["image"].(string)
		ready, _ := csMap["ready"].(bool)
		restartCount := toInt64(csMap["restartCount"])
		state, reason := containerStateAndReason(csMap)
		result = append(result, map[string]interface{}{
			"name":         name,
			"image":        image,
			"state":        state,
			"reason":       reason,
			"ready":        ready,
			"restartCount": restartCount,
		})
	}
	return result
}

// containerStateAndReason extracts the state name and reason from a containerStatus map.
func containerStateAndReason(csMap map[string]interface{}) (state, reason string) {
	stateMap, _ := csMap["state"].(map[string]interface{})
	if stateMap == nil {
		return "", ""
	}
	if _, ok := stateMap["running"]; ok {
		return "Running", ""
	}
	if w, ok := stateMap["waiting"].(map[string]interface{}); ok {
		r, _ := w["reason"].(string)
		return "Waiting", r
	}
	if t, ok := stateMap["terminated"].(map[string]interface{}); ok {
		r, _ := t["reason"].(string)
		return "Terminated", r
	}
	return "", ""
}

// podExitCode returns the exit code from the first terminated container.
// Returns -1 when no container has terminated.
func podExitCode(status map[string]interface{}) int64 {
	if status == nil {
		return -1
	}
	containerStatuses, _ := status["containerStatuses"].([]interface{})
	for _, cs := range containerStatuses {
		csMap, _ := cs.(map[string]interface{})
		if csMap == nil {
			continue
		}
		state, _ := csMap["state"].(map[string]interface{})
		if state == nil {
			continue
		}
		terminated, _ := state["terminated"].(map[string]interface{})
		if terminated == nil {
			continue
		}
		return toInt64(terminated["exitCode"])
	}
	return -1
}

// podOrdinal extracts the ordinal from a StatefulSet pod name (<name>-<ordinal>).
// Returns -1 for non-ordinal pods (Deployment, Job, etc.).
func podOrdinal(name string) int64 {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return -1
	}
	n, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func isPodReady(status map[string]interface{}) bool {
	if status == nil {
		return false
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, c := range conditions {
		cond, _ := c.(map[string]interface{})
		if cond == nil {
			continue
		}
		if t, _ := cond["type"].(string); t == "Ready" {
			s, _ := cond["status"].(string)
			return s == "True"
		}
	}
	return false
}

func podTotalRestarts(status map[string]interface{}) int64 {
	if status == nil {
		return 0
	}
	containerStatuses, _ := status["containerStatuses"].([]interface{})
	var total int64
	for _, cs := range containerStatuses {
		csMap, _ := cs.(map[string]interface{})
		if csMap == nil {
			continue
		}
		total += toInt64(csMap["restartCount"])
	}
	return total
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
