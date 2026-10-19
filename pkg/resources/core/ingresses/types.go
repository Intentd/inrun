// pkg/resources/ingresses/types.go
package ingresses

// Spec is the fully resolved Ingress specification.
// All template expressions have been evaluated before this struct is populated.
// Passed directly to Create, Apply and Delete.
type Spec struct {
	// Name — Ingress resource name. Required.
	Name string

	// Namespace — target namespace.
	Namespace string

	// Host — virtual host for the Ingress rule.
	Host string

	// ServiceName — backend Service name.
	ServiceName string

	// ServicePort — backend Service port.
	ServicePort int32

	// Path — HTTP path prefix. Default: "/"
	Path string

	// PathType — Prefix | Exact | ImplementationSpecific. Default: Prefix.
	PathType string

	// IngressClass — Ingress class name. Empty means cluster default.
	IngressClass string

	// Labels applied to Ingress metadata.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels map[string]string

	// Annotations applied to Ingress metadata.
	Annotations map[string]string

	// TLS — optional TLS configuration. nil means no TLS.
	TLS *TLS
}

// TLS holds the fully resolved TLS configuration for an Ingress.
type TLS struct {
	// Create — whether to configure TLS on this Ingress.
	Create bool

	// SecretName — name of the kubernetes.io/tls Secret.
	SecretName string

	// Hosts — hostnames covered by the TLS certificate.
	Hosts []string

	// ValidFor — certificate validity duration string passed to GenerateTLSBundle.
	ValidFor string
}
