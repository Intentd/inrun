// pkg/types/security.go
//
// Security configuration at the Catalog level.
//
// The unified security block covers four concerns:
//
//  1. Deletion protection — webhook that blocks deletion of managed CRDs and the operator itself.
//  2. Admission webhooks  — ValidatingWebhookConfiguration and MutatingWebhookConfiguration.
//  3. Conversion          — CRD version conversion via /convert (separate from webhooks).
//
// YAML shape:
//
//	security:
//	  deletionProtection:
//	    enabled: true            # default: true when block is present
//	    serviceName: inrun    # default: INRUN_SERVICE_NAME env / "inrun"
//	    failurePolicy: Fail      # default: Fail
//
//	  webhooks:
//	    admission:
//	      enabled: true          # default: ENABLE_ADMISSION_WEBHOOK env / false
//	    failurePolicy: Ignore    # default: WEBHOOKS_FAILURE_POLICY env / "Ignore"
//	    serviceName: inrun    # default: INRUN_SERVICE_NAME env / "inrun"
//
//	  conversion:
//	    enabled: true            # default: ENABLE_CONVERSION env / false
//	    conversionWindow: 100    # default: CONVERSION_WINDOW env / 100
//
// Precedence: catalog YAML value > ENV value > hard default.
// ENV values populate SecurityConfig during Init() and act as defaults.
// Catalog values are merged on top in BuildRuntimeCatalog.
package types

// CertManagerConfig controls Inrun's built-in TLS certificate lifecycle.
// Only applies when certificates are auto-generated (no TLS_CERT/TLS_KEY env vars).
//
// YAML shape:
//
//	security:
//	  certManager:
//	    autoRotate: true           # default: true — rotate cert before expiry
//	    rotationThreshold: "30d"   # default: 30 days before expiry
type CertManagerConfig struct {
	// AutoRotate controls whether Inrun pre-emptively rotates the TLS certificate
	// before it expires. The new certificate takes effect on the next gateway restart.
	// Default: true. Set to false or TLS_AUTO_ROTATE=false to opt out.
	AutoRotate *bool `yaml:"autoRotate,omitempty" json:"autoRotate,omitempty"`

	// RotationThreshold is how far before expiry Inrun rotates the certificate.
	// Accepts duration strings: "30d", "7d", "2w". Default: "30d".
	RotationThreshold string `yaml:"rotationThreshold,omitempty" json:"rotationThreshold,omitempty"`

	// ValidFor is the validity period of the certificate.
	// Accepts duration strings: "30d", "7d", "2w". Default: "1y".
	ValidFor string `yaml:"validFor,omitempty" json:"validFor,omitempty"`
}

// CatalogSecurity holds the full security configuration for a Catalog.
type CatalogSecurity struct {
	// ServiceName defines the runtime and gateway service names for the Inrun deployment.
	ServiceName *ServiceName `yaml:"serviceName,omitempty" json:"serviceName,omitempty"`

	// DeletionProtection controls whether Inrun registers a webhook that
	// blocks deletion of its managed CRDs, deployment, service, etc.
	//
	// When enabled (default when block is present):
	//   - Registers /deletion-protection endpoint on the HTTPS server
	//   - Creates ValidatingWebhookConfiguration "inrun-deletion-protection"
	//   - Entry 1: broad rule for CRDs; handler filters by ProtectedCRDNames()
	//   - Entry 2: ObjectSelector-gated rule for deployment, service, etc
	//
	// To decommission an operator with deletion protection:
	//   1. Set enabled: false
	//   2. Redeploy Inrun (webhook removed on startup)
	//   3. Delete resources normally
	//
	// nil pointer: not enabled (not declared in YAML).
	DeletionProtection *DeletionProtectionConfig `yaml:"deletionProtection,omitempty" json:"deletionProtection,omitempty"`

	// Webhooks controls the admission webhook settings (ValidatingWebhookConfiguration,
	// MutatingWebhookConfiguration). These apply globally — there is no per-CRD switch.
	// CRDs declare validation/mutation rules; the webhook is enabled or not globally.
	//
	// nil pointer: admission webhooks not configured; ENV vars drive behavior.
	Webhooks *WebhooksConfig `yaml:"webhooks,omitempty" json:"webhooks,omitempty"`

	// Conversion controls the /convert endpoint and CRD version conversion.
	// Separate from admission webhooks — conversion has its own endpoint and
	// configuration (window size for stats, etc.).
	//
	// nil pointer: conversion not configured; ENV vars drive behavior.
	Conversion *ConversionConfig `yaml:"conversion,omitempty" json:"conversion,omitempty"`

	// NamespaceProtection controls the optional validating webhook that prevents
	// Inrun-managed CRs from being created or updated in forbidden namespaces.
	//
	// This is an admission-time safeguard only. When enabled:
	//   - Registers /namespace-protection endpoint on the HTTPS server
	//   - Creates ValidatingWebhookConfiguration "inrun-namespace-protection"
	//   - Enforces allowedNamespaces / restrictedNamespaces declared by each CRD
	//
	// If disabled or omitted, namespace rules are not enforced at apply time.
	// Existing CRs in forbidden namespaces will still be reconciled normally.
	//
	// nil pointer: namespace protection not configured; ENV vars drive behavior.
	NamespaceProtection *NamespaceProtectionConfig `yaml:"namespaceProtection,omitempty" json:"namespaceProtection,omitempty"`

	// CertManager controls the lifecycle of Inrun's auto-generated TLS certificate.
	// Only applies when certificates are auto-generated (TLS_CERT/TLS_KEY not set).
	//
	// nil pointer: use ENV defaults (TLS_AUTO_ROTATE, TLS_ROTATION_THRESHOLD).
	CertManager *CertManagerConfig `yaml:"certManager,omitempty" json:"certManager,omitempty"`
}

// ServiceName defines the canonical names used to reference a service within
// Inrun. Runtime is the internal name used inside the operator, while
// Gateway is the externally exposed name used by ingress or gateway layers.
// Both fields are optional and omitted when empty.
type ServiceName struct {
	// Runtime is the internal service name used by the operator and runtime
	// components. This is typically the name used for wiring and enrichment.
	Runtime string `json:"runtime,omitempty" yaml:"runtime,omitempty"`

	// Gateway is the gateway's Service. Every webhook Inrun registers
	// (deletion and namespace protection, admission, conversion) points at it,
	// and the generated TLS certificate is issued for it.
	Gateway string `json:"gateway,omitempty" yaml:"gateway,omitempty"`
}

// DeletionProtectionConfig controls deletion protection behaviour.
type DeletionProtectionConfig struct {
	// Enabled controls whether deletion protection is active.
	// Default: true when the deletionProtection block is declared.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// FailurePolicy controls what the API server does when Inrun is unreachable.
	// "Fail" — reject the DELETE (recommended for protection; this is the default).
	// "Ignore" — allow the DELETE through when Inrun cannot be reached.
	// Default: "Fail".
	FailurePolicy string `yaml:"failurePolicy,omitempty" json:"failurePolicy,omitempty"`

	// CleanupOnShutdown controls whether Deletion protection webhook is deleted on graceful shutdown.
	// Default: false — Deletion protection webhook persists across restarts.
	CleanupOnShutdown bool `yaml:"cleanupOnShutdown,omitempty" json:"cleanupOnShutdown,omitempty"`

	// StrictMode controls whether removing the deletion-protection label from a resource
	// is itself treated as a deletion attempt and blocked.
	// When true, the only way to remove the label (and thus unprotect a resource) is to
	// disable strictMode in the Catalog and restart Inrun Gateway.
	// Default: false.
	StrictMode bool `yaml:"strictMode,omitempty" json:"strictMode,omitempty"`
}

// NamespaceProtectionConfig controls namespace-protection webhook behaviour.
type NamespaceProtectionConfig struct {
	// Enabled controls whether namespace protection is active.
	// Default: true when the namespaceProtection block is declared.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// FailurePolicy controls what the API server does when Inrun is unreachable.
	// "Fail"   — reject the CREATE/UPDATE (recommended; this is the default).
	// "Ignore" — allow the request through when Inrun cannot be reached.
	// Default: "Fail".
	FailurePolicy string `yaml:"failurePolicy,omitempty" json:"failurePolicy,omitempty"`

	// RestrictedNamespaces — deny-list applied to every CRD in this Catalog.
	// Merged additively with per-CRD restrictedNamespaces — more specific levels
	// add to, not replace, broader levels.
	RestrictedNamespaces RestrictedNamespaces `yaml:"restrictedNamespaces,omitempty" json:"restrictedNamespaces,omitempty"`

	// AllowedNamespaces — allow-list applied to every CRD in this Catalog.
	// Merged additively with per-CRD allowedNamespaces.
	AllowedNamespaces AllowedNamespaces `yaml:"allowedNamespaces,omitempty" json:"allowedNamespaces,omitempty"`

	// CleanupOnShutdown controls whether Namespace protection webhook is deleted on graceful shutdown.
	// Default: false — Namespace protection webhook persists across restarts.
	CleanupOnShutdown bool `yaml:"cleanupOnShutdown,omitempty" json:"cleanupOnShutdown,omitempty"`
}

// WebhooksConfig controls global admission webhook settings.
// Conversion is declared separately under security.conversion.
type WebhooksConfig struct {
	// Admission controls the ValidatingWebhookConfiguration and MutatingWebhookConfiguration.
	Admission *AdmissionWebhookToggle `yaml:"admission,omitempty" json:"admission,omitempty"`

	// FailurePolicy controls what the API server does when Inrun is unreachable
	// for admission calls. "Fail" or "Ignore".
	// Default: WEBHOOKS_FAILURE_POLICY env / "Ignore".
	FailurePolicy string `yaml:"failurePolicy,omitempty" json:"failurePolicy,omitempty"`

	// CleanupOnShutdown controls whether Admission webhook is deleted on graceful shutdown.
	// Default: false — Admission webhook persists across restarts.
	CleanupOnShutdown bool `yaml:"cleanupOnShutdown,omitempty" json:"cleanupOnShutdown,omitempty"`
}

// AdmissionWebhookToggle controls whether admission webhooks are globally enabled.
type AdmissionWebhookToggle struct {
	// Enabled controls whether ValidatingWebhookConfiguration and
	// MutatingWebhookConfiguration are registered at startup.
	// Default: ENABLE_ADMISSION_WEBHOOK env / false.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// ConversionConfig controls the /convert endpoint and CRD version conversion.
type ConversionConfig struct {
	// Enabled controls whether the /convert endpoint is registered and the
	// CRD conversion webhook is active.
	// Default: ENABLE_CONVERSION env / false.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// ConversionWindow is the rolling window size for latency/throughput stats.
	// Default: CONVERSION_WINDOW env / 100.
	ConversionWindow int `yaml:"conversionWindow,omitempty" json:"conversionWindow,omitempty"`
}

// ── Effective value helpers ───────────────────────────────────────────────────

// IsDeletionProtectionEnabled returns the effective deletion protection setting.
func (s *CatalogSecurity) IsDeletionProtectionEnabled() bool {
	if s == nil || s.DeletionProtection == nil {
		return false
	}
	if s.DeletionProtection.Enabled == nil {
		return true // declared but no explicit value = enabled
	}
	return *s.DeletionProtection.Enabled
}

// IsAdmissionEnabled returns true when admission webhooks are globally enabled.
func (s *CatalogSecurity) IsAdmissionEnabled() bool {
	if s == nil || s.Webhooks == nil || s.Webhooks.Admission == nil {
		return false
	}
	if s.Webhooks.Admission.Enabled == nil {
		return false // no default-on for webhooks — must be explicit
	}
	return *s.Webhooks.Admission.Enabled
}

// IsConversionEnabled returns true when the conversion webhook is globally enabled.
func (s *CatalogSecurity) IsConversionEnabled() bool {
	if s == nil || s.Conversion == nil {
		return false
	}
	if s.Conversion.Enabled == nil {
		return false
	}
	return *s.Conversion.Enabled
}

// RuntimeServiceName returns the effective service name for inrun runtime.
// Falls back to the provided ENV default.
func (s *CatalogSecurity) RuntimeServiceName(envDefault string) string {
	if s != nil && s.ServiceName != nil && s.ServiceName.Runtime != "" {
		return s.ServiceName.Runtime
	}
	return envDefault
}

// GatewayServiceName returns the effective service name for inrun gateway.
// Falls back to the provided ENV default.
func (s *CatalogSecurity) GatewayServiceName(envDefault string) string {
	if s != nil && s.ServiceName != nil && s.ServiceName.Gateway != "" {
		return s.ServiceName.Gateway
	}
	return envDefault
}

// DeletionProtectionFailurePolicy returns the effective failure policy string.
// Falls back to "Fail" when not configured — protecting by default.
func (s *CatalogSecurity) DeletionProtectionFailurePolicy() string {
	if s != nil && s.DeletionProtection != nil && s.DeletionProtection.FailurePolicy != "" {
		return s.DeletionProtection.FailurePolicy
	}
	return "Fail"
}

// IsNamespaceProtectionEnabled returns the effective namespace protection setting.
func (s *CatalogSecurity) IsNamespaceProtectionEnabled() bool {
	if s == nil || s.NamespaceProtection == nil {
		return false
	}
	if s.NamespaceProtection.Enabled == nil {
		return true // declared but no explicit value = enabled
	}
	return *s.NamespaceProtection.Enabled
}

// NamespaceProtectionFailurePolicy returns the effective failure policy string.
// Falls back to "Fail" when not configured — protecting by default.
func (s *CatalogSecurity) NamespaceProtectionFailurePolicy() string {
	if s != nil && s.NamespaceProtection != nil && s.NamespaceProtection.FailurePolicy != "" {
		return s.NamespaceProtection.FailurePolicy
	}
	return "Fail"
}

// WebhooksFailurePolicy returns the effective failure policy for admission webhooks.
// Falls back to "Ignore" — not blocking when Inrun is unreachable.
func (s *CatalogSecurity) WebhooksFailurePolicy(envDefault string) string {
	if s != nil && s.Webhooks != nil && s.Webhooks.FailurePolicy != "" {
		return s.Webhooks.FailurePolicy
	}
	if envDefault != "" {
		return envDefault
	}
	return "Ignore"
}

// IsCertAutoRotateEnabled returns the effective auto-rotate setting.
// Default: true — rotation is on unless explicitly disabled.
func (s *CatalogSecurity) IsCertAutoRotateEnabled() bool {
	if s == nil || s.CertManager == nil || s.CertManager.AutoRotate == nil {
		return true
	}
	return *s.CertManager.AutoRotate
}

// CertRotationThresholdVal returns the effective rotation threshold string.
// Falls back to the provided ENV default.
func (s *CatalogSecurity) CertRotationThresholdVal(envDefault string) string {
	if s != nil && s.CertManager != nil && s.CertManager.RotationThreshold != "" {
		return s.CertManager.RotationThreshold
	}
	return envDefault
}

// ValidForVal returns the effective validity string.
// Falls back to the provided ENV default.
func (s *CatalogSecurity) ValidForVal(envDefault string) string {
	if s != nil && s.CertManager != nil && s.CertManager.ValidFor != "" {
		return s.CertManager.ValidFor
	}
	return envDefault
}
