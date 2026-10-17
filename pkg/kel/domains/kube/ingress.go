package note

import (
	"strings"
	"text/template"
)

// ingressNotes registers helpers for inspecting Ingress status and spec fields.
//
// Usage:
//
//	tmpl.Funcs(note.ingressNotes())
//
// Template examples:
//
//	{{ ingressReady .children.ingress }}
//	{{ ingressHost .children.ingress }}
//	{{ ingressIP .children.ingress }}
//	{{ ingressClassName .children.ingress }}
//	{{ ingressRules .children.ingress }}
//	{{ ingressTLSHosts .children.ingress }}
//
// No enrichment required — all notes navigate the Ingress object directly.
func ingressNotes() template.FuncMap {
	return template.FuncMap{
		"ingressReady":     noteIngressReady,
		"ingressHost":      noteIngressHost,
		"ingressIP":        noteIngressIP,
		"ingressClassName": noteIngressClassName,
		"ingressRules":     noteIngressRules,
		"ingressTLSHosts":  noteIngressTLSHosts,
		// Enriched ingress notes — require enrich: [ingress] on the CRD.
		"ingressLoadBalancerIPs": noteIngressLoadBalancerIPs,
		"ingressTLSSecretCount":  noteIngressTLSSecretCount,
	}
}

// ── Ingress notes ─────────────────────────────────────────────────────────────

// noteIngressReady returns true when at least one load balancer ingress entry
// has been assigned — either an IP or a hostname.
//
//	{{ ingressReady .children.ingress }}
func noteIngressReady(obj interface{}) bool {
	entry := ingressLBEntry(obj)
	if entry == nil {
		return false
	}
	ip, _ := entry["ip"].(string)
	host, _ := entry["hostname"].(string)
	return ip != "" || host != ""
}

// noteIngressHost returns the hostname assigned by the load balancer
// (status.loadBalancer.ingress[0].hostname).
//
//	{{ ingressHost .children.ingress }}
//	→ "abc123.us-east-1.elb.amazonaws.com"
func noteIngressHost(obj interface{}) string {
	entry := ingressLBEntry(obj)
	if entry == nil {
		return ""
	}
	v, _ := entry["hostname"].(string)
	return v
}

// noteIngressIP returns the external IP assigned by the load balancer
// (status.loadBalancer.ingress[0].ip).
//
//	{{ ingressIP .children.ingress }}  → "34.123.45.67"
func noteIngressIP(obj interface{}) string {
	entry := ingressLBEntry(obj)
	if entry == nil {
		return ""
	}
	v, _ := entry["ip"].(string)
	return v
}

// notIngressClassName returns the ingress class name (spec.ingressClassName).
// Empty when not set.
//
//	{{ ingressClassName .children.ingress }}  → "nginx"
func noteIngressClassName(obj interface{}) string {
	spec := noteSpec(obj)
	if spec == nil {
		return ""
	}
	v, _ := spec["ingressClassName"].(string)
	return v
}

// noteIngressRules returns a comma-separated list of hostnames from spec.rules.
// Empty hosts (catch-all rules) are omitted.
//
//	{{ ingressRules .children.ingress }}  → "api.example.com, www.example.com"
func noteIngressRules(obj interface{}) string {
	spec := noteSpec(obj)
	if spec == nil {
		return ""
	}
	rules, _ := spec["rules"].([]interface{})
	var hosts []string
	for _, r := range rules {
		rm, _ := r.(map[string]interface{})
		if rm == nil {
			continue
		}
		if h, _ := rm["host"].(string); h != "" {
			hosts = append(hosts, h)
		}
	}
	return strings.Join(hosts, ", ")
}

// noteIngressTLSHosts returns a comma-separated list of TLS hostnames from spec.tls.
//
//	{{ ingressTLSHosts .children.ingress }}  → "api.example.com, www.example.com"
func noteIngressTLSHosts(obj interface{}) string {
	spec := noteSpec(obj)
	if spec == nil {
		return ""
	}
	tlsEntries, _ := spec["tls"].([]interface{})
	var hosts []string
	seen := map[string]bool{}
	for _, t := range tlsEntries {
		tm, _ := t.(map[string]interface{})
		if tm == nil {
			continue
		}
		tlsHosts, _ := tm["hosts"].([]interface{})
		for _, h := range tlsHosts {
			if s, _ := h.(string); s != "" && !seen[s] {
				seen[s] = true
				hosts = append(hosts, s)
			}
		}
	}
	return strings.Join(hosts, ", ")
}

// ── Enriched Ingress notes ────────────────────────────────────────────────────

// noteIngressLoadBalancerIPs reads _loadBalancerIPs and returns a comma-joined list of IPs or hostnames.
// Requires enrich: [ingress] on the CRD.
//
//	{{ ingressLoadBalancerIPs .children.ingress }}  → "1.2.3.4, example.com"
func noteIngressLoadBalancerIPs(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	ips, _ := m["_loadBalancerIPs"].([]interface{})
	var parts []string
	for _, entry := range ips {
		if s, _ := entry.(string); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// noteIngressTLSSecretCount reads _tlsSecrets and returns the count.
// Requires enrich: [ingress] on the CRD.
//
//	{{ ingressTLSSecretCount .children.ingress }}  → 1
func noteIngressTLSSecretCount(obj interface{}) int {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return 0
	}
	secrets, _ := m["_tlsSecrets"].([]interface{})
	return len(secrets)
}

// Helpers
func ingressLBEntry(obj interface{}) map[string]interface{} {
	status := noteStatus(obj)
	lb, _ := status["loadBalancer"].(map[string]interface{})
	if lb == nil {
		return nil
	}
	ingress, _ := lb["ingress"].([]interface{})
	if len(ingress) == 0 {
		return nil
	}
	m, _ := ingress[0].(map[string]interface{})
	return m
}
