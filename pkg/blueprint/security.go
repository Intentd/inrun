// pkg/blueprint/security.go
//
// Security accessors on *Blueprint.
//
// Precedence (highest → lowest):
//  1. Blueprint YAML (k.Security)
//  2. ENV vars via SecurityConfig (k.config.Security())
//  3. Hard defaults coded below
//
// BlueprintSecurity uses *bool fields which allows detecting
// "not declared" (nil) vs "explicitly false" (*false).
//
// Deletion protection is ENABLED BY DEFAULT when the security.deletionProtection block
// is present but deletionProtection.enabled is not declared.
package blueprint

import (
	"time"
)

// securityEnvDefaults returns the SecurityConfig from config, or a zero
// value when config is not wired (e.g. NewEmptyBlueprint in tests).
func (c *Blueprint) securityEnvDefaults() interface {
	RuntimeServiceName() string
	GatewayServiceName() string
	DeletionProtectionEnabled() bool
	DeletionProtectionPolicy() string
	DeletionProtectionCleanup() bool
	AdmissionEnabled() bool
	ConversionEnabled() bool
	WebhooksPolicy() string
	WebhookCleanup() bool
	ConversionWindowVal() int
	HousekeeperEnabled() bool
	HousekeeperSyncInterval() time.Duration
	NamespaceProtectionEnabled() bool
	NamespaceProtectionPolicy() string
	NamespaceProtectionCleanup() bool
	CertAutoRotate() bool
	CertValidForStr() string
	CertRotationThresholdStr() string
} {
	return &envSecurityReader{bp: c}
}

const (
	inrunRun  = "inrun-runtime"
	inrunGate = "inrun-gateway"
)

// envSecurityReader adapts *config.SecurityConfig through a small interface
// so that security.go does not import config directly.
type envSecurityReader struct{ bp *Blueprint }

func (r *envSecurityReader) RuntimeServiceName() string {
	if r.bp.config == nil {
		return inrunRun
	}
	return r.bp.config.Security().ServiceName.Runtime
}

func (r *envSecurityReader) GatewayServiceName() string {
	if r.bp.config == nil {
		return inrunGate
	}
	return r.bp.config.Security().ServiceName.Gateway
}

func (r *envSecurityReader) DeletionProtectionEnabled() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().DeletionProtection.Enabled
}
func (r *envSecurityReader) DeletionProtectionPolicy() string {
	if r.bp.config == nil {
		return "Fail"
	}
	return r.bp.config.Security().DeletionProtection.FailurePolicy
}
func (r *envSecurityReader) AdmissionEnabled() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().Webhooks.Admission.Enabled
}
func (r *envSecurityReader) ConversionEnabled() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().Conversion.Enabled
}
func (r *envSecurityReader) WebhooksPolicy() string {
	if r.bp.config == nil {
		return "Ignore"
	}
	return r.bp.config.Security().Webhooks.FailurePolicy
}

func (r *envSecurityReader) WebhookCleanup() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().Webhooks.CleanupOnShutdown
}
func (r *envSecurityReader) DeletionProtectionCleanup() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().DeletionProtection.CleanupOnShutdown
}
func (r *envSecurityReader) NamespaceProtectionCleanup() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().NamespaceProtection.CleanupOnShutdown
}

func (r *envSecurityReader) ConversionWindowVal() int {
	if r.bp.config == nil {
		return 100
	}
	return r.bp.config.Security().Conversion.ConversionWindow
}
func (r *envSecurityReader) HousekeeperEnabled() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().Webhooks.Housekeeper.Enabled
}
func (r *envSecurityReader) HousekeeperSyncInterval() time.Duration {
	if r.bp.config == nil {
		return 30 * time.Second
	}
	return r.bp.config.Security().Webhooks.Housekeeper.SyncInterval
}
func (r *envSecurityReader) NamespaceProtectionEnabled() bool {
	if r.bp.config == nil {
		return false
	}
	return r.bp.config.Security().NamespaceProtection.Enabled
}
func (r *envSecurityReader) NamespaceProtectionPolicy() string {
	if r.bp.config == nil {
		return "Fail"
	}
	return r.bp.config.Security().NamespaceProtection.FailurePolicy
}

func (r *envSecurityReader) CertAutoRotate() bool {
	if r.bp.config == nil {
		return true
	}
	return r.bp.config.Security().CertManager.AutoRotate
}

func (r *envSecurityReader) CertRotationThresholdStr() string {
	if r.bp.config == nil {
		return "30d"
	}
	return r.bp.config.Security().CertManager.RotationThreshold
}

func (r *envSecurityReader) CertValidForStr() string {
	if r.bp.config == nil {
		return "1y"
	}
	return r.bp.config.Security().CertManager.ValidFor
}

//

// ── Deletion protection ───────────────────────────────────────────────────────

// IsDeletionProtectionEnabled reports whether deletion protection is active.
//
// Precedence:
//
//	YAML security.deletionProtection block present → use YAML value (default-on when block declared)
//	YAML block absent                              → fall back to ENABLE_DELETION_PROTECTION env
func (c *Blueprint) IsDeletionProtectionEnabled() bool {
	if c.Security.DeletionProtection != nil {
		// YAML declared the block — apply its enabled semantics.
		return c.Security.IsDeletionProtectionEnabled()
	}
	return c.securityEnvDefaults().DeletionProtectionEnabled()
}

// Runtime Service Name returns the effective service name for inrun runtime.
// YAML value takes precedence over ENV.
func (c *Blueprint) RuntimeServiceName() string {
	env := c.securityEnvDefaults()
	return c.Security.RuntimeServiceName(env.RuntimeServiceName())
}

// Gateway Service Name returns the effective service name for inrun gateway.
// YAML value takes precedence over ENV.
func (c *Blueprint) GatewayServiceName() string {
	env := c.securityEnvDefaults()
	return c.Security.GatewayServiceName(env.GatewayServiceName())
}

// DeletionProtectionFailurePolicy returns the effective failure policy string.
// YAML value takes precedence over ENV.
func (c *Blueprint) DeletionProtectionFailurePolicy() string {
	if c.Security.DeletionProtection != nil && c.Security.DeletionProtection.FailurePolicy != "" {
		return c.Security.DeletionProtection.FailurePolicy
	}
	return c.securityEnvDefaults().DeletionProtectionPolicy()
}

// IsStrictModeEnabled reports whether strict mode is active for deletion protection.
// Strict mode treats removal of the deletion-protection label as a deletion attempt.
// Only valid when deletion protection is also enabled.
func (c *Blueprint) IsStrictModeEnabled() bool {
	if c.Security.DeletionProtection == nil {
		return false
	}
	return c.Security.DeletionProtection.StrictMode
}

// ── Namespace protection ───────────────────────────────────────────────────────

// IsNamespaceProtectionEnabled reports whether namespace protection is active.
//
// Precedence:
//
//	YAML security.namespaceProtection block present → use YAML value (default-on when block declared)
//	YAML block absent                               → fall back to ENABLE_NAMESPACE_PROTECTION env
func (c *Blueprint) IsNamespaceProtectionEnabled() bool {
	if c.Security.NamespaceProtection != nil {
		// YAML declared the block — apply its enabled semantics.
		return c.Security.IsNamespaceProtectionEnabled()
	}
	return c.securityEnvDefaults().NamespaceProtectionEnabled()
}

// NamespaceProtectionFailurePolicy returns the effective failure policy string.
// YAML value takes precedence over ENV.
func (c *Blueprint) NamespaceProtectionFailurePolicy() string {
	if c.Security.NamespaceProtection != nil && c.Security.NamespaceProtection.FailurePolicy != "" {
		return c.Security.NamespaceProtection.FailurePolicy
	}
	return c.securityEnvDefaults().NamespaceProtectionPolicy()
}

// NamespaceProtectionCleanupOnShutdown reports whether Deletion Protection should be deleted on shutdown.
//
// Precedence:
//
//	YAML security.namespaceProtection.cleanupOnShutdown present → use YAML value
//	YAML block absent                                      → fall back to NAMESPACE_PROTECTION_CLEANUP_ON_SHUTDOWN env
func (c *Blueprint) NamespaceProtectionCleanupOnShutdown() bool {
	if c.Security.NamespaceProtection != nil {
		return c.Security.NamespaceProtection.CleanupOnShutdown
	}
	return c.securityEnvDefaults().NamespaceProtectionCleanup()
}

// ── Admission webhooks ────────────────────────────────────────────────────────

// IsAdmissionEnabled reports whether admission webhooks are globally enabled.
//
// Precedence:
//
//	YAML security.webhooks.admission block present → use YAML value
//	YAML block absent                              → fall back to ENABLE_ADMISSION_WEBHOOK env
func (c *Blueprint) IsAdmissionEnabled() bool {
	if c.Security.Webhooks != nil && c.Security.Webhooks.Admission != nil {
		return c.Security.IsAdmissionEnabled()
	}
	return c.securityEnvDefaults().AdmissionEnabled()
}

// ── Conversion webhook ────────────────────────────────────────────────────────

// IsConversionEnabled reports whether the conversion webhook is globally enabled.
//
// Precedence:
//
//	YAML security.conversion block present → use YAML value
//	YAML block absent                      → fall back to ENABLE_CONVERSION env
func (c *Blueprint) IsConversionEnabled() bool {
	if c.Security.Conversion != nil {
		return c.Security.IsConversionEnabled()
	}
	return c.securityEnvDefaults().ConversionEnabled()
}

// WebhooksFailurePolicy returns the effective failure policy for admission webhooks.
// YAML value takes precedence over ENV.
func (c *Blueprint) WebhooksFailurePolicy() string {
	env := c.securityEnvDefaults()
	return c.Security.WebhooksFailurePolicy(env.WebhooksPolicy())
}

// DeletionProtectionCleanupOnShutdown reports whether Deletion Protection should be deleted on shutdown.
//
// Precedence:
//
//	YAML security.deletionProtection.cleanupOnShutdown present → use YAML value
//	YAML block absent                                      → fall back to DELETION_PROTECTION_CLEANUP_ON_SHUTDOWN env
func (c *Blueprint) DeletionProtectionCleanupOnShutdown() bool {
	if c.Security.DeletionProtection != nil {
		return c.Security.DeletionProtection.CleanupOnShutdown
	}
	return c.securityEnvDefaults().DeletionProtectionCleanup()
}

// WebhookCleanupOnShutdown reports whether admission webhooks should be deleted on shutdown.
//
// Precedence:
//
//	YAML security.webhooks.cleanupOnShutdown present → use YAML value
//	YAML block absent                                → fall back to WEBHOOK_CLEANUP_ON_SHUTDOWN env
func (c *Blueprint) WebhookCleanupOnShutdown() bool {
	if c.Security.Webhooks != nil {
		return c.Security.Webhooks.CleanupOnShutdown
	}
	return c.securityEnvDefaults().WebhookCleanup()
}

// ── Conversion stats window ───────────────────────────────────────────────────

// ConversionWindow returns the effective rolling window size for conversion/admission stats.
//
// Precedence:
//
//	YAML security.conversion.conversionWindow > 0 → use YAML value
//	YAML absent or zero                           → fall back to CONVERSION_WINDOW env
func (c *Blueprint) ConversionWindow() int {
	if c.Security.Conversion != nil && c.Security.Conversion.ConversionWindow > 0 {
		return c.Security.Conversion.ConversionWindow
	}
	return c.securityEnvDefaults().ConversionWindowVal()
}

// ── Gateway config ────────────────────────────────────────────────────────────

// IsGatewayEnabled reports whether this Blueprint declares a gateway: block at
// all — deliberately just a nil check, not a check of individual fields
// within it (enabled, endpoint, api, …). Any declared gateway: block
// means the blueprint expects a gateway to exist; checking specific fields
// undercounts as the block grows new standalone-meaningful sections (e.g.
// api: with no enabled: or endpoint: set, as in a CI-only Gateway API
// client setup).
func (c *Blueprint) IsGatewayEnabled() bool {
	if c == nil {
		return false
	}
	return c.Gateway != nil
}

// IsGatewayAPIEnabled reports whether the Gateway Gateway API flag is set.
func (c *Blueprint) IsGatewayAPIEnabled() bool {
	if c == nil {
		return false
	}
	return c.Gateway != nil && c.Gateway.API != nil && c.Gateway.API.Enabled
}

// HasGatewayAPISecretRefs reports whether any Gateway API token entry uses secretRef.
// Used by GenerateGatewayRBACRules to add secrets get/create permissions.
func (c *Blueprint) HasGatewayAPISecretRefs() bool {
	if !c.IsGatewayAPIEnabled() {
		return false
	}
	for _, t := range c.Gateway.API.Auth.Tokens {
		if t.SecretRef != nil {
			return true
		}
	}
	return false
}

// HasServeEnabled reports whether the Gateway API is enabled and at least one CRD
// has serve.enabled: true. This is the activation gate — callers should use this,
// not IsGatewayAPIEnabled, to decide whether to register serve routes or RBAC.
func (c *Blueprint) HasServeEnabled() bool {
	if c == nil {
		return false
	}
	if !c.IsGatewayAPIEnabled() {
		return false
	}
	for _, crd := range c.Enabled() {
		if crd.ServeEnabled() {
			return true
		}
	}
	return false
}

// IsStandaloneGateway reports whether this Blueprint is deployed as a standalone
// gateway with no companion runtime operator.
//
// When true:
//   - gatewayEndpoint validation is skipped
//   - spec: may be empty (no CRDs required)
func (c *Blueprint) IsStandaloneGateway() bool {
	return c.Gateway != nil && c.Gateway.Standalone
}

// GatewayEndpoint returns the effective gateway endpoint URL.
//
// Precedence:
//
//	YAML gateway.endpoint non-empty          → use gateway block value
//	If not set          	                 → fall back to INRUN_GATEWAY_ENDPOINT env
func (c *Blueprint) GatewayEndpoint() string {
	if c.Gateway != nil && c.Gateway.Endpoint != "" {
		return c.Gateway.Endpoint
	}
	if c.config != nil {
		return c.config.GatewayEndpoint()
	}
	return ""
}

// ClusterName returns the effective cluster name for this Blueprint.
//
// Precedence:
//
//	metadata.clusterName non-empty → use Blueprint value
//	CLUSTER_NAME env var set       → use config value
//	Neither set                    → inrun-runtime
func (c *Blueprint) ClusterName() string {
	if c.metadata.ClusterName != "" {
		return c.metadata.ClusterName
	}
	if c.config != nil {
		return c.config.Cluster().Name()
	}
	return inrunRun
}

// ── Gateway requirement ───────────────────────────────────────────────────────

// NeedsGateway reports whether this Blueprint requires a companion gateway process.
//
// A gateway is required when any of the following are configured:
//   - Security features that run on the gateway's HTTPS server
//     (deletion protection, admission webhooks, conversion, namespace protection)
//   - Notifications — unless standalone: true is declared (or implied by local dev)
//
// Used by Validate to fail fast when gatewayEndpoint is not set.
func (c *Blueprint) NeedsGateway() bool {
	return c.NeedsCertificates()
}

// ── Certificates ──────────────────────────────────────────────────────────────

// NeedsCertificates reports whether Inrun must generate TLS certificates.
//
// Certificates are required when deletion protection, admission webhooks,
// or conversion webhooks are enabled with valid usecases
// configured in at least 1 CRD— all three use the same TLS cert.
func (c *Blueprint) NeedsCertificates() bool {
	return c.IsDeletionProtectionEnabled() ||
		c.IsNamespaceProtectionEnabled() ||
		c.HasValidationOrMutationRules() || // Valid admission rule
		c.HasConversionPaths()
}

// IsHousekeeperEnabled reports whether the webhook housekeeper is enabled.
func (c *Blueprint) IsHousekeeperEnabled() bool {
	return c.securityEnvDefaults().HousekeeperEnabled()
}

// HousekeeperSyncInterval returns the housekeeper sync interval.
func (c *Blueprint) HousekeeperSyncInterval() time.Duration {
	return c.securityEnvDefaults().HousekeeperSyncInterval()
}

// ── Certificate manager ───────────────────────────────────────────────────────

// CertAutoRotate reports whether pre-emptive TLS certificate rotation is enabled.
//
// Precedence:
//
//	YAML security.certManager.autoRotate declared → use YAML value
//	YAML absent                                   → fall back to TLS_AUTO_ROTATE env (default: true)
func (c *Blueprint) CertAutoRotate() bool {
	if c.Security.CertManager != nil {
		return c.Security.IsCertAutoRotateEnabled()
	}
	return c.securityEnvDefaults().CertAutoRotate()
}

// CertRotationThreshold returns the pre-rotation window as a parsed duration.
// Returns 30 days when the configured value cannot be parsed.
//
// Precedence:
//
//	YAML security.certManager.rotationThreshold non-empty → use YAML value
//	YAML absent or empty                                  → fall back to TLS_ROTATION_THRESHOLD env (default: "30d")
func (c *Blueprint) CertRotationThreshold() time.Duration {
	raw := c.Security.CertRotationThresholdVal(c.securityEnvDefaults().CertRotationThresholdStr())
	if d, err := parseTimeDuration(raw); err == nil {
		return d
	}
	return 30 * 24 * time.Hour
}

// CertValidFor returns the certificate validity duration as a parsed duration.
// Returns 1 year when the configured value cannot be parsed.
//
// Precedence:
//
//	YAML security.certManager.rotateAfter non-empty → use YAML value
//	YAML absent or empty                                  → fall back to TLS_ROTATE_AFTER env (default: "1y")
func (c *Blueprint) CertValidFor() time.Duration {
	raw := c.Security.ValidForVal(c.securityEnvDefaults().CertValidForStr())
	if d, err := parseTimeDuration(raw); err == nil {
		return d
	}
	return 365 * 24 * time.Hour
}

// CertValidForStr returns the raw validity string for use in CertificateSpec.ValidFor.
// Falls back to "1y" when not configured.
func (c *Blueprint) CertValidForStr() string {
	return c.Security.ValidForVal(c.securityEnvDefaults().CertValidForStr())
}

// GatewayTokenNames returns the names of all tokens declared in
// gateway.api.auth.tokens. Returns nil when the gateway is not enabled.
func (c *Blueprint) GatewayTokenNames() []string {
	if !c.IsGatewayEnabled() {
		return nil
	}
	if !c.Gateway.HasAPI() || c.Gateway.API.Auth.Empty() {
		return nil
	}
	names := make([]string, 0, len(c.Gateway.API.Auth.Tokens))
	for _, t := range c.Gateway.API.Auth.Tokens {
		names = append(names, t.Name)
	}
	return names
}
