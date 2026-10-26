package children

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithStorageClass fetches the StorageClass for each PVC and embeds
// it under "_storageClass". A no-op when storageclass enrichment is not enabled
// on the CRD.
func enrichGroupWithStorageClass(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("storageclass", crd) {
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
		scName, _ := spec["storageClassName"].(string)
		if scName == "" {
			continue
		}
		u, err := kube.DynamicClient().
			Resource(StorageClassGVR).
			Get(ctx, scName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		obj["_storageClass"] = u.Object
	}
}
