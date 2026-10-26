package children

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithPV embeds the bound PV object under "_pv" for each PVC in
// the group. A no-op when pvc enrichment is not enabled on the CRD.
// The PV name comes from spec.volumeName on the PVC, set by Kubernetes once bound.
func enrichGroupWithPV(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("pv", crd) {
		return
	}

	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		spec, _ := obj["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}
		volName, _ := spec["volumeName"].(string)
		if volName == "" {
			continue
		}
		u, err := kube.DynamicClient().
			Resource(PersistentVolumeGVR).
			Get(ctx, volName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		pvObj := u.Object
		if s, ok := pvObj["status"]; !ok || s == nil {
			pvObj["status"] = map[string]interface{}{}
		}
		obj["_pv"] = pvObj
	}
}
