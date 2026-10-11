package children

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithEndpoints embeds _endpoints into each service map.
// Reads the EndpointSlice for each service and embeds IP:port pairs.
// A no-op when endpoint enrichment is not enabled on the CRD.
func enrichGroupWithEndpoints(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("endpoints", crd) {
		return
	}
	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		meta, _ := obj["metadata"].(map[string]interface{})
		ns, svcName := "", ""
		if meta != nil {
			ns, _ = meta["namespace"].(string)
			svcName, _ = meta["name"].(string)
		}
		if svcName == "" {
			continue
		}
		enrichServiceWithEndpoints(ctx, kube, ns, svcName, obj)
	}
}

// enrichServiceWithEndpoints fetches the EndpointSlice for the named service
// and embeds a list of {ip, port, ready} maps under _endpoints.
func enrichServiceWithEndpoints(ctx context.Context, kube kubeclient.Interface, ns, svcName string, obj map[string]interface{}) {
	list, err := kube.DynamicClient().
		Resource(EndpointSliceGVR).
		Namespace(ns).
		List(ctx, metav1.ListOptions{
			LabelSelector:   fmt.Sprintf("kubernetes.io/service-name=%s", svcName),
			Limit:           1,
			ResourceVersion: "0",
		})
	if err != nil || list == nil || len(list.Items) == 0 {
		return
	}
	endpoints := extractEndpointEntries(list.Items[0].Object)
	if len(endpoints) > 0 {
		obj["_endpoints"] = endpoints
	}
}

// extractEndpointEntries builds the flat _endpoints list from an EndpointSlice object.
func extractEndpointEntries(esObj map[string]interface{}) []interface{} {
	ports := extractSlicePorts(esObj)
	eps, _ := esObj["endpoints"].([]interface{})

	var result []interface{}
	for _, e := range eps {
		em, _ := e.(map[string]interface{})
		if em == nil {
			continue
		}
		cond, _ := em["conditions"].(map[string]interface{})
		ready, _ := cond["ready"].(bool)

		addrs, _ := em["addresses"].([]interface{})
		for _, addr := range addrs {
			ip, _ := addr.(string)
			if ip == "" {
				continue
			}
			for _, port := range ports {
				result = append(result, map[string]interface{}{
					"ip":    ip,
					"port":  port,
					"ready": ready,
				})
			}
		}
	}
	return result
}

// extractSlicePorts returns the port numbers from an EndpointSlice's ports array.
func extractSlicePorts(esObj map[string]interface{}) []int64 {
	portObjs, _ := esObj["ports"].([]interface{})
	ports := make([]int64, 0, len(portObjs))
	for _, p := range portObjs {
		pm, _ := p.(map[string]interface{})
		if pm == nil {
			continue
		}
		ports = append(ports, toInt64(pm["port"]))
	}
	return ports
}
