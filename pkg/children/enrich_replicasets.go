package children

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithReplicaSets lists ReplicaSets owned by each Deployment and
// attaches them (native) as a slice under "_replicaSets".
// A no-op when replicasets enrichment is not enabled on the CRD.
func enrichGroupWithReplicaSets(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !crd.ShouldEnrich("replicasets") {
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
		uid, _ := meta["uid"].(string)
		if ns == "" || uid == "" {
			continue
		}
		list, err := kube.DynamicClient().
			Resource(ReplicaSetGVR).
			Namespace(ns).
			List(ctx, metav1.ListOptions{ResourceVersion: "0"})
		if err != nil || list == nil {
			continue
		}
		var rsList []interface{}
		for i := range list.Items {
			if objectOwnedByUID(list.Items[i].Object, uid) {
				rsList = append(rsList, list.Items[i].Object)
			}
		}
		if len(rsList) > 0 {
			obj["_replicaSets"] = rsList
		}
	}
}

// activeReplicaSetGroup extracts the active (current) ReplicaSet from each
// deployment's _replicaSets and returns a name→object map suitable for use as
// children["replicasets"]. The active RS is the one with the highest
// spec.replicas — typically the RS managed by the current rollout.
// Returns nil when no deployments have _replicaSets embedded.
func activeReplicaSetGroup(deployments map[string]interface{}) map[string]interface{} {
	for _, v := range deployments {
		dep, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		rsList, _ := dep["_replicaSets"].([]interface{})
		if len(rsList) == 0 {
			continue
		}
		// Pick the RS with the highest spec.replicas (the active one).
		var best map[string]interface{}
		var bestReplicas int64 = -1
		for _, raw := range rsList {
			rs, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			spec, _ := rs["spec"].(map[string]interface{})
			replicas := toInt64(spec["replicas"])
			if replicas > bestReplicas {
				bestReplicas = replicas
				best = rs
			}
		}
		if best == nil {
			best, _ = rsList[0].(map[string]interface{})
		}
		if best == nil {
			continue
		}
		meta, _ := best["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)
		if name == "" {
			continue
		}
		return map[string]interface{}{name: best}
	}
	return nil
}

// objectOwnedByUID returns true when the object has an ownerReference whose
// uid matches the given uid and controller is true.
func objectOwnedByUID(obj map[string]interface{}, uid string) bool {
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
		if u, _ := r["uid"].(string); u == uid {
			return true
		}
	}
	return false
}
