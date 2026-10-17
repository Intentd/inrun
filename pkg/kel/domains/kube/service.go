package note

import (
	"fmt"
	"strings"
	"text/template"
)

// serviceNotes registers helpers for inspecting Service networking fields and
// endpoint readiness.
//
// Usage:
//
//	tmpl.Funcs(note.serviceNotes())
//
// Template examples:
//
//	{{ serviceClusterIP .children.service }}
//	{{ serviceNodePort .children.service "http" }}
//	{{ serviceLoadBalancerIP .children.service }}
//	{{ serviceLoadBalancerHost .children.service }}
//	{{ endpointsReady .children.endpointslice }}
//
// These helpers provide a concise way to surface clusterIP, nodePort,
// load‑balancer details, and endpoint readiness—useful for gating traffic
// routing, validating exposure, or ensuring downstream components only proceed
// once a Service is fully addressable.
func serviceNotes() template.FuncMap {
	return template.FuncMap{
		"serviceClusterIP":        noteServiceClusterIP,
		"serviceNodePort":         noteServiceNodePort,
		"serviceLoadBalancerIP":   noteServiceLoadBalancerIP,
		"serviceLoadBalancerHost": noteServiceLoadBalancerHost,
		"endpointsReady":          noteEndpointsReady,
		// Enriched endpoint notes — require enrich: [endpoints] on the CRD.
		"hasEndpoints":         noteHasEndpoints,
		"serviceEndpoints":     noteServiceEndpoints,
		"serviceEndpointCount": noteServiceEndpointCount,
		"serviceFirstEndpoint": noteServiceFirstEndpoint,
		// Enriched backing-pod notes — require enrich: [backingpods] on the CRD.
		"backingPodCount": noteBackingPodCount,
		"backingPodNames": noteBackingPodNames,
	}
}

// ── Enriched endpoint notes ───────────────────────────────────────────────────

// noteHasEndpoints returns true when at least one ready endpoint exists in _endpoints.
// Requires enrich: [endpoints] on the CRD.
//
//	{{ hasEndpoints .children.service }}
func noteHasEndpoints(obj interface{}) bool {
	for _, e := range getEndpoints(obj) {
		ep, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		if ready, _ := ep["ready"].(bool); ready {
			return true
		}
	}
	return false
}

// noteServiceEndpoints returns all ready endpoints as a comma-separated "ip:port" list.
// Requires enrich: [endpoints] on the CRD.
//
//	{{ serviceEndpoints .children.service }}  → "10.0.0.1:8080, 10.0.0.2:8080"
func noteServiceEndpoints(obj interface{}) string {
	var parts []string
	for _, e := range getEndpoints(obj) {
		ep, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		ip, _ := ep["ip"].(string)
		port := toInt64(ep["port"])
		if ip == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", ip, port))
	}
	return strings.Join(parts, ", ")
}

// noteServiceEndpointCount returns the total number of endpoints in _endpoints.
// Requires enrich: [endpoints] on the CRD.
//
//	{{ serviceEndpointCount .children.service }}  → 3
func noteServiceEndpointCount(obj interface{}) int {
	return len(getEndpoints(obj))
}

// noteServiceFirstEndpoint returns the first endpoint as "ip:port".
// Returns "" when no endpoints exist.
// Requires enrich: [endpoints] on the CRD.
//
//	{{ serviceFirstEndpoint .children.service }}  → "10.0.0.1:8080"
func noteServiceFirstEndpoint(obj interface{}) string {
	eps := getEndpoints(obj)
	if len(eps) == 0 {
		return ""
	}
	ep, ok := eps[0].(map[string]interface{})
	if !ok {
		return ""
	}
	ip, _ := ep["ip"].(string)
	port := toInt64(ep["port"])
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func getEndpoints(obj interface{}) []interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}
	eps, _ := m["_endpoints"].([]interface{})
	return eps
}

// ── Service notes ─────────────────────────────────────────────────────────────

// noteServiceClusterIP returns the assigned ClusterIP for a Service.
// Returns "" before the IP is assigned or when the Service doesn't exist.
//
//	{{ serviceClusterIP .children.service }}  → "10.96.0.1"
func noteServiceClusterIP(obj interface{}) string {
	spec := noteGet(obj, "spec")
	m, ok := spec.(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := m["clusterIP"].(string)
	return v
}

// noteServiceNodePort returns the NodePort for the first port of a Service.
// Returns 0 before the NodePort is assigned.
// Use containerPortByName for named ports.
//
//	{{ serviceNodePort .children.service }}  → 31234
func noteServiceNodePort(obj interface{}) int {
	spec := noteGet(obj, "spec")
	m, ok := spec.(map[string]interface{})
	if !ok {
		return 0
	}
	ports, ok := m["ports"].([]interface{})
	if !ok || len(ports) == 0 {
		return 0
	}
	pm, ok := ports[0].(map[string]interface{})
	if !ok {
		return 0
	}
	return int(toInt64(pm["nodePort"]))
}

// noteServiceLoadBalancerIP returns the external IP assigned by the load balancer.
// Returns "" when not yet assigned (common — LB provisioning takes time).
//
//	{{ serviceLoadBalancerIP .children.service }}  → "34.123.45.67"
//
// Use in status fields to surface the external IP once available:
//
//   - path: externalIP
//     value: "{{ serviceLoadBalancerIP .children.service }}"
func noteServiceLoadBalancerIP(obj interface{}) string {
	ingress := lbIngress(obj)
	if ingress == nil {
		return ""
	}
	ip, _ := ingress["ip"].(string)
	return ip
}

// noteServiceLoadBalancerHost returns the hostname assigned by the load balancer.
// Cloud providers typically assign a hostname rather than an IP.
//
//	{{ serviceLoadBalancerHost .children.service }}
//	→ "abc123.us-east-1.elb.amazonaws.com"
func noteServiceLoadBalancerHost(obj interface{}) string {
	ingress := lbIngress(obj)
	if ingress == nil {
		return ""
	}
	host, _ := ingress["hostname"].(string)
	return host
}

func lbIngress(obj interface{}) map[string]interface{} {
	status := noteStatus(obj)
	lb, ok := status["loadBalancer"].(map[string]interface{})
	if !ok {
		return nil
	}
	ingress, ok := lb["ingress"].([]interface{})
	if !ok || len(ingress) == 0 {
		return nil
	}
	m, _ := ingress[0].(map[string]interface{})
	return m
}

// noteEndpointsReady returns true when at least one endpoint address is ready.
// Accepts an EndpointSlice object (discovery.k8s.io/v1), auto-fetched as
// .children.endpointslice for any Service declared in onCreate.
//
//	{{ endpointsReady .children.endpointslice }}
//
// EndpointSlice format (checked first):
//
//	endpoints[].conditions.ready == true && endpoints[].addresses non-empty
//
// Legacy Endpoints format (fallback):
//
//	subsets[].addresses non-empty
func noteEndpointsReady(obj interface{}) bool {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return false
	}

	// EndpointSlice format (discovery.k8s.io/v1)
	if eps, ok := m["endpoints"].([]interface{}); ok {
		for _, e := range eps {
			em, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			addrs, _ := em["addresses"].([]interface{})
			if len(addrs) == 0 {
				continue
			}
			cond, _ := em["conditions"].(map[string]interface{})
			ready, _ := cond["ready"].(bool)
			if ready {
				return true
			}
		}
		return false
	}

	// Legacy Endpoints format (v1): subsets at top level or under status
	for _, s := range legacyEndpointSubsets(m) {
		sm, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		addrs, ok := sm["addresses"].([]interface{})
		if ok && len(addrs) > 0 {
			return true
		}
	}
	return false
}

func legacyEndpointSubsets(m map[string]interface{}) []interface{} {
	if subsets, ok := m["subsets"].([]interface{}); ok {
		return subsets
	}
	if status, ok := m["status"].(map[string]interface{}); ok {
		if subsets, ok := status["subsets"].([]interface{}); ok {
			return subsets
		}
	}
	return nil
}

// ── Enriched backing-pod notes ────────────────────────────────────────────────

// noteBackingPodCount reads _backingPods and returns the count.
// Requires enrich: [backingpods] on the CRD.
//
//	{{ backingPodCount .children.service }}  → 3
func noteBackingPodCount(obj interface{}) int {
	return len(getBackingPods(obj))
}

// noteBackingPodNames reads _backingPods[*].name and returns a comma-joined list.
// Requires enrich: [backingpods] on the CRD.
//
//	{{ backingPodNames .children.service }}  → "app-abc, app-def"
func noteBackingPodNames(obj interface{}) string {
	var names []string
	for _, p := range getBackingPods(obj) {
		pm, _ := p.(map[string]interface{})
		if pm == nil {
			continue
		}
		if name, _ := pm["name"].(string); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func getBackingPods(obj interface{}) []interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}
	pods, _ := m["_backingPods"].([]interface{})
	return pods
}
