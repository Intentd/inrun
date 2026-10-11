package children

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithStatefulSetPVCs fetches the PVCs for each StatefulSet and
// embeds them as a slice under "_pvcs". A no-op when pvcs enrichment is not
// enabled on the CRD.
//
// StatefulSet PVC names are deterministic: <templateName>-<stsName>-<ordinal>.
// This lets us fetch each PVC directly without a List+filter.
func enrichGroupWithStatefulSetPVCs(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("pvcs", crd) {
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
		stsName, _ := meta["name"].(string)
		if ns == "" || stsName == "" {
			continue
		}

		spec, _ := obj["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}

		// Default replica count is 1 when spec.replicas is unset.
		replicas := int(toInt64(spec["replicas"]))
		if replicas == 0 {
			replicas = 1
		}

		vcts, _ := spec["volumeClaimTemplates"].([]interface{})
		if len(vcts) == 0 {
			continue
		}

		var pvcs []interface{}
		for _, vct := range vcts {
			vctMap, _ := vct.(map[string]interface{})
			if vctMap == nil {
				continue
			}
			vctMeta, _ := vctMap["metadata"].(map[string]interface{})
			if vctMeta == nil {
				continue
			}
			templateName, _ := vctMeta["name"].(string)
			if templateName == "" {
				continue
			}
			for ordinal := 0; ordinal < replicas; ordinal++ {
				pvcName := fmt.Sprintf("%s-%s-%d", templateName, stsName, ordinal)
				u, err := kube.DynamicClient().
					Resource(PersistentVolumeClaimGVR).
					Namespace(ns).
					Get(ctx, pvcName, metav1.GetOptions{})
				if err != nil {
					continue
				}
				pvcs = append(pvcs, u.Object)
			}
		}
		if len(pvcs) > 0 {
			obj["_pvcs"] = pvcs
		}
	}
}
