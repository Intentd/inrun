package types

// TLSSpec declares TLS certificate generation for a secret.
// When set, the secret is created as type kubernetes.io/tls with
// tls.crt, tls.key, and ca.crt fields.
type TLSSpec struct {
	// CommonName is the certificate's CN field.
	//   commonName: "{{ .metadata.name }}.{{ .metadata.namespace }}.svc"
	CommonName string `yaml:"commonName" json:"commonName"`

	// DNSNames are the Subject Alternative Names.
	// Template expressions supported per entry.
	DNSNames []string `yaml:"dnsNames,omitempty" json:"dnsNames,omitempty"`

	// ValidFor is the certificate validity period.
	// Default: same as rotateAfter, or 1y if rotateAfter is not set.
	// Must be >= rotateAfter to avoid immediate expiry on rotation.
	ValidFor string `yaml:"validFor,omitempty" json:"validFor,omitempty"`

	// Organization is the cert's O field. Default: "inrun"
	Organization string `yaml:"organization,omitempty" json:"organization,omitempty"`
}
