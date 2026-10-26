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

// enrichServiceWithEndpoints attaches the named service's EndpointSlices
// (native) under _endpoints. A Service can have several slices.
func enrichServiceWithEndpoints(ctx context.Context, kube kubeclient.Interface, ns, svcName string, obj map[string]interface{}) {
	list, err := kube.DynamicClient().
		Resource(EndpointSliceGVR).
		Namespace(ns).
		List(ctx, metav1.ListOptions{
			LabelSelector:   fmt.Sprintf("kubernetes.io/service-name=%s", svcName),
			ResourceVersion: "0",
		})
	if err != nil || list == nil || len(list.Items) == 0 {
		return
	}
	slices := make([]interface{}, 0, len(list.Items))
	for i := range list.Items {
		slices = append(slices, list.Items[i].Object)
	}
	obj["_endpoints"] = slices
}
