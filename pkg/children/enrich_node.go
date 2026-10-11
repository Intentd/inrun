package children

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithNode fetches the Node for each Pod and embeds a summary under
// "_node". A no-op when node enrichment is not enabled on the CRD.
//
// Summary: {name, zone, region, instanceType}
// Zone and region come from well-known topology labels.
func enrichGroupWithNode(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("node", crd) {
		return
	}

	nodeCache := map[string]interface{}{}
	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		spec, _ := obj["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}
		nodeName, _ := spec["nodeName"].(string)
		if nodeName == "" {
			continue
		}
		// Cache to avoid fetching the same node multiple times when
		// a catalog declares multiple pods on the same node.
		if cached, ok := nodeCache[nodeName]; ok {
			obj["_node"] = cached
			continue
		}
		u, err := kube.DynamicClient().
			Resource(NodeGVR).
			Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		summary := buildNodeSummary(u.Object)
		nodeCache[nodeName] = summary
		obj["_node"] = summary
	}
}

// buildNodeSummary extracts the fields most useful for status expressions.
// Topology labels follow the well-known Kubernetes label schema:
// topology.kubernetes.io/zone and topology.kubernetes.io/region.
// Instance type comes from node.kubernetes.io/instance-type (cloud providers).
func buildNodeSummary(nodeObj map[string]interface{}) map[string]interface{} {
	meta, _ := nodeObj["metadata"].(map[string]interface{})
	name := ""
	zone, region, instanceType := "", "", ""
	if meta != nil {
		name, _ = meta["name"].(string)
		labels, _ := meta["labels"].(map[string]interface{})
		if labels != nil {
			zone, _ = labels["topology.kubernetes.io/zone"].(string)
			region, _ = labels["topology.kubernetes.io/region"].(string)
			instanceType, _ = labels["node.kubernetes.io/instance-type"].(string)
		}
	}
	return map[string]interface{}{
		"name":         name,
		"zone":         zone,
		"region":       region,
		"instanceType": instanceType,
	}
}
