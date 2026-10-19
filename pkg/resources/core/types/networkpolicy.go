package types

// NetworkPolicyIngressRule describes one ingress rule.
type NetworkPolicyIngressRule struct {
	From  []NetworkPolicyPeer `yaml:"from,omitempty" json:"from,omitempty"`
	Ports []NetworkPolicyPort `yaml:"ports,omitempty" json:"ports,omitempty"`
}

// NetworkPolicyEgressRule describes one egress rule.
type NetworkPolicyEgressRule struct {
	To    []NetworkPolicyPeer `yaml:"to,omitempty" json:"to,omitempty"`
	Ports []NetworkPolicyPort `yaml:"ports,omitempty" json:"ports,omitempty"`
}

// NetworkPolicyPeer selects a set of pods or namespaces.
type NetworkPolicyPeer struct {
	// PodSelector selects pods within the namespace. Empty = all pods.
	PodSelector map[string]string `yaml:"podSelector,omitempty" json:"podSelector,omitempty"`
	// NamespaceSelector selects namespaces. Empty = all namespaces.
	NamespaceSelector map[string]string `yaml:"namespaceSelector,omitempty" json:"namespaceSelector,omitempty"`
	// IPBlock selects a CIDR range.
	IPBlock *NetworkPolicyIPBlock `yaml:"ipBlock,omitempty" json:"ipBlock,omitempty"`
}

// NetworkPolicyIPBlock describes an IP CIDR range.
type NetworkPolicyIPBlock struct {
	CIDR   string   `yaml:"cidr" json:"cidr"`
	Except []string `yaml:"except,omitempty" json:"except,omitempty"`
}

// NetworkPolicyPort describes a port allowed by a rule.
type NetworkPolicyPort struct {
	// Protocol — TCP (default), UDP, or SCTP.
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	// Port — port number or named port. Supports template expressions.
	Port string `yaml:"port,omitempty" json:"port,omitempty"`
}
