package children

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithIngressData embeds load-balancer IPs and TLS secret objects
// for each Ingress in the group. A no-op when ingress enrichment is not enabled
// on the CRD.
//
// _loadBalancerIPs: flat list of IP/hostname strings from status.loadBalancer.ingress.
// _tlsSecrets: list of full Secret objects named in spec.tls[*].secretName.
func enrichGroupWithIngressData(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("ingress", crd) {
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

		// _loadBalancerIPs from status.loadBalancer.ingress
		status, _ := obj["status"].(map[string]interface{})
		if status != nil {
			lb, _ := status["loadBalancer"].(map[string]interface{})
			if lb != nil {
				ingressList, _ := lb["ingress"].([]interface{})
				var ips []interface{}
				for _, entry := range ingressList {
					em, _ := entry.(map[string]interface{})
					if em == nil {
						continue
					}
					if ip, _ := em["ip"].(string); ip != "" {
						ips = append(ips, ip)
					} else if host, _ := em["hostname"].(string); host != "" {
						ips = append(ips, host)
					}
				}
				if len(ips) > 0 {
					obj["_loadBalancerIPs"] = ips
				}
			}
		}

		// _tlsSecrets from spec.tls[*].secretName
		if ns == "" {
			continue
		}
		spec, _ := obj["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}
		tlsList, _ := spec["tls"].([]interface{})
		var secrets []interface{}
		seen := map[string]bool{}
		for _, tls := range tlsList {
			tlsMap, _ := tls.(map[string]interface{})
			if tlsMap == nil {
				continue
			}
			secretName, _ := tlsMap["secretName"].(string)
			if secretName == "" || seen[secretName] {
				continue
			}
			seen[secretName] = true
			u, err := kube.DynamicClient().
				Resource(SecretGVR).
				Namespace(ns).
				Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				continue
			}
			secrets = append(secrets, u.Object)
		}
		if len(secrets) > 0 {
			obj["_tlsSecrets"] = secrets
		}
	}
}
