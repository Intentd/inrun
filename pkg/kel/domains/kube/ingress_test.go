// pkg/note/kube_ingress_test.go
package note

import (
	"testing"
)

func makeIngress(lbIP, lbHost string, rules, tlsHosts []string) map[string]interface{} {
	lbEntry := map[string]interface{}{"ip": lbIP, "hostname": lbHost}
	ruleList := make([]interface{}, 0, len(rules))
	for _, h := range rules {
		ruleList = append(ruleList, map[string]interface{}{"host": h})
	}
	tlsList := []interface{}{}
	if len(tlsHosts) > 0 {
		hosts := make([]interface{}, 0, len(tlsHosts))
		for _, h := range tlsHosts {
			hosts = append(hosts, h)
		}
		tlsList = []interface{}{map[string]interface{}{"hosts": hosts}}
	}
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"rules": ruleList,
			"tls":   tlsList,
		},
		"status": map[string]interface{}{
			"loadBalancer": map[string]interface{}{
				"ingress": []interface{}{lbEntry},
			},
		},
	}
}

func TestNoteIngressReady(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"no status", map[string]interface{}{}, false},
		{"no lb", map[string]interface{}{"status": map[string]interface{}{}}, false},
		{"lb with ip", makeIngress("34.1.2.3", "", nil, nil), true},
		{"lb with hostname", makeIngress("", "abc.elb.amazonaws.com", nil, nil), true},
		{"lb empty both", makeIngress("", "", nil, nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressReady(tt.obj); got != tt.want {
				t.Errorf("noteIngressReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteIngressHost(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no lb", map[string]interface{}{}, ""},
		{"with hostname", makeIngress("", "abc.elb.amazonaws.com", nil, nil), "abc.elb.amazonaws.com"},
		{"ip only no hostname", makeIngress("34.1.2.3", "", nil, nil), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressHost(tt.obj); got != tt.want {
				t.Errorf("noteIngressHost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteIngressIP(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no lb", map[string]interface{}{}, ""},
		{"with ip", makeIngress("34.1.2.3", "", nil, nil), "34.1.2.3"},
		{"hostname only no ip", makeIngress("", "abc.elb.amazonaws.com", nil, nil), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressIP(tt.obj); got != tt.want {
				t.Errorf("noteIngressIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteIngressRules(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no spec", map[string]interface{}{}, ""},
		{"no rules", makeIngress("", "", nil, nil), ""},
		{"one rule", makeIngress("", "", []string{"api.example.com"}, nil), "api.example.com"},
		{"two rules", makeIngress("", "", []string{"api.example.com", "www.example.com"}, nil), "api.example.com, www.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressRules(tt.obj); got != tt.want {
				t.Errorf("noteIngressRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteIngressTLSHosts(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no tls", makeIngress("", "", nil, nil), ""},
		{"one tls host", makeIngress("", "", nil, []string{"api.example.com"}), "api.example.com"},
		{"two tls hosts", makeIngress("", "", nil, []string{"api.example.com", "www.example.com"}), "api.example.com, www.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressTLSHosts(tt.obj); got != tt.want {
				t.Errorf("noteIngressTLSHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func makeIngressWithEnrichment(lbIPs []string, tlsSecretCount int) map[string]interface{} {
	ips := make([]interface{}, len(lbIPs))
	for i, ip := range lbIPs {
		ips[i] = ip
	}
	secrets := make([]interface{}, tlsSecretCount)
	for i := range secrets {
		secrets[i] = map[string]interface{}{"metadata": map[string]interface{}{"name": "secret"}}
	}
	return map[string]interface{}{
		"_loadBalancerIPs": ips,
		"_tlsSecrets":      secrets,
	}
}

func TestNoteIngressLoadBalancerIPs(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"nil", nil, ""},
		{"no enrichment", map[string]interface{}{}, ""},
		{"one IP", makeIngressWithEnrichment([]string{"1.2.3.4"}, 0), "1.2.3.4"},
		{"IP and hostname", makeIngressWithEnrichment([]string{"1.2.3.4", "example.com"}, 0), "1.2.3.4, example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressLoadBalancerIPs(tt.obj); got != tt.want {
				t.Errorf("noteIngressLoadBalancerIPs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteIngressTLSSecretCount(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want int
	}{
		{"nil", nil, 0},
		{"no enrichment", map[string]interface{}{}, 0},
		{"one secret", makeIngressWithEnrichment(nil, 1), 1},
		{"two secrets", makeIngressWithEnrichment(nil, 2), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIngressTLSSecretCount(tt.obj); got != tt.want {
				t.Errorf("noteIngressTLSSecretCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
