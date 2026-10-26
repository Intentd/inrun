// pkg/types/blueprint.go
package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// GatewayConfig declares how the Inrun gateway is deployed for this Blueprint.
//
// YAML shape:
//
//	gateway:
//	  enabled: true     # explicitly enable gateway installation
//	  standalone: true  # gateway runs without a companion runtime operator
//	  endpoint: ""      # leave empty when standalone; sets this when paired with runtime
//	  api:
//	    enabled: true
//	    auth:
//	      tokens:
//	        - name: ci-pipeline
//	          secretRef:
//	            name: inrun-apply-token
//	            key: token
//	            rotateAfter: 90d
type GatewayConfig struct {
	// Enabled declares whether the gateway should be installed for this blueprint.
	// When true, this means:
	//   - Helm installation was done with '--set gateway.enabled=true'
	//   - The runtime expects a gateway to exist
	// Default: false.
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Standalone declares that this Blueprint is deployed as a gateway-only installation
	// with no companion runtime operator. When true:
	//   - gatewayEndpoint validation is skipped (the gateway is self-contained)
	//   - spec: may be empty (no CRDs required)
	// Default: false.
	Standalone bool `yaml:"standalone,omitempty" json:"standalone,omitempty"`

	// Endpoint is the HTTP base URL of the gateway, used by the runtime to locate it.
	// Leave empty in standalone deployments.
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`

	// API enables the CRUD REST surface for CRs on this gateway.
	API *GatewayAPIConfig `yaml:"api,omitempty" json:"api,omitempty"`

	// Clusters registers named remote clusters the gateway may route intents to.
	// When absent, all intents apply to the local cluster (default behaviour).
	// Keys are cluster names referenced by serve.cluster and target.cluster.
	// Supports an optional "include:" key at the clusters level — same pattern
	// as gateway.webhooks.include and gateway.api.auth.include.
	Clusters *GatewayClustersConfig `yaml:"clusters,omitempty" json:"clusters,omitempty"`
}

// GatewayClustersConfig holds the named remote cluster map and an optional
// include path. The YAML form is a mapping whose keys are either "include"
// (the path to a clusters: file) or cluster names with GatewayClusterConfig values.
//
//	gateway:
//	  clusters:
//	    include: ./clusters.yaml
//	    prod:
//	      endpoint: https://prod.internal:6443
//	      secretRef:
//	        name: prod-creds
//	        key: kubeconfig
type GatewayClustersConfig struct {
	// Include is a path (relative to the blueprint file) to a YAML file whose
	// top-level "clusters:" key holds additional GatewayClusterConfig entries.
	// Included entries load first; inline entries override by name.
	// Cleared after expansion.
	Include string

	// Entries maps cluster names to their connection config.
	// Populated from the inline keys and/or the include file.
	Entries map[string]GatewayClusterConfig
}

// UnmarshalYAML handles the mixed "include" + cluster-name keys at the clusters level.
func (c *GatewayClustersConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("gateway.clusters must be a mapping")
	}
	if c.Entries == nil {
		c.Entries = make(map[string]GatewayClusterConfig)
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch key {
		case "include":
			c.Include = val.Value
		default:
			var cfg GatewayClusterConfig
			if err := val.Decode(&cfg); err != nil {
				return fmt.Errorf("clusters[%q]: %w", key, err)
			}
			c.Entries[key] = cfg
		}
	}
	return nil
}

// MarshalYAML serialises as a plain map (include is cleared after expansion).
func (c GatewayClustersConfig) MarshalYAML() (interface{}, error) {
	m := make(map[string]interface{}, len(c.Entries)+1)
	if c.Include != "" {
		m["include"] = c.Include
	}
	for name, cfg := range c.Entries {
		m[name] = cfg
	}
	return m, nil
}

// MarshalJSON serialises as the flat entries map (include cleared by expansion).
func (c GatewayClustersConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Entries)
}

// UnmarshalJSON deserialises from the flat entries map.
func (c *GatewayClustersConfig) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &c.Entries)
}

// GatewayClusterConfig holds the connection details for one registered remote cluster.
// Exactly one credential form must be declared: secretRef (kubeconfig) or
// tokenRef + caRef (bearer token + CA cert, the ArgoCD pattern).
type GatewayClusterConfig struct {
	// Endpoint is the Kubernetes API server URL for this cluster.
	// Example: https://prod.internal:6443
	Endpoint string `yaml:"endpoint" json:"endpoint"`

	// SecretRef locates a Kubernetes Secret whose data key holds a kubeconfig.
	// Mutually exclusive with TokenRef + CARef.
	// The kubeconfig may use any auth method internally (token, client cert,
	// exec plugin, OIDC). Use this when you already manage kubeconfigs.
	SecretRef *APISecretRef `yaml:"secretRef,omitempty" json:"secretRef,omitempty"`

	// TokenRef locates a Kubernetes Secret whose data key holds a bearer token
	// (typically a service account token) for this cluster.
	// Must be paired with CARef. Mutually exclusive with SecretRef.
	// This is the ArgoCD credential pattern — prefer it when you want explicit
	// least-privilege service account scoping rather than a full kubeconfig.
	TokenRef *APISecretRef `yaml:"tokenRef,omitempty" json:"tokenRef,omitempty"`

	// CARef locates a Kubernetes Secret whose data key holds the cluster's CA
	// certificate (PEM, base64-encoded). Used together with TokenRef to verify
	// the API server's TLS certificate.
	// Must be paired with TokenRef. Mutually exclusive with SecretRef.
	CARef *APISecretRef `yaml:"caRef,omitempty" json:"caRef,omitempty"`

	// Insecure skips TLS verification when connecting to this cluster.
	// Only valid with TokenRef (kubeconfig manages TLS settings internally).
	// Use only in local development — never in production.
	Insecure bool `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

// APIConfig enables and configures the Gateway Gateway API.
type GatewayAPIConfig struct {
	// Enabled activates the Gateway API handlers on the gateway.
	// When true, the gateway registers POST /api/v1/apply,
	// GET/DELETE /api/v1/resources/..., and GET /api/v1/schema/... routes.
	// Default: false.
	Enabled bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Auth configures bearer token authentication for Gateway API requests.
	Auth APIAuth `yaml:"auth,omitempty" json:"auth,omitempty"`
}

// APIAuth holds the token list for Gateway API authentication.
type APIAuth struct {
	// Tokens is the list of accepted bearer tokens. Every Gateway API request
	// must include Authorization: Bearer <token> matching one entry.
	Tokens []APIToken `yaml:"tokens,omitempty" json:"tokens,omitempty"`

	// Include is a path (relative to the blueprint file) to a YAML file with a
	// "tokens:" list (same shape as the inline tokens below). Expanded at load
	// time — the result is merged into Tokens, with inline entries taking
	// precedence per token name.
	Include string `yaml:"include,omitempty" json:"include,omitempty"`
}

// APIToken is one bearer token entry.
// Exactly one of Token, SecretRef, GitHubOIDC, GitLabOIDC, VaultOIDC, or OIDC must be set.
type APIToken struct {
	// Name is a human-readable identifier used in logs and audit output.
	Name string `yaml:"name" json:"name"`

	// SecretRef reads the token value from a Kubernetes Secret at startup.
	// If the Secret does not exist, the gateway creates it with a generated
	// uuidv4 token. If rotateAfter is set, the gateway rotates the token
	// using the same annotation-based rotation as pkg/runners.
	SecretRef *APISecretRef `yaml:"secretRef,omitempty" json:"secretRef,omitempty"`

	// Token is an ${ENV_VAR} reference expanded at startup.
	// Set the variable via extraEnv in the gateway and console Helm values.
	// Literal values are not accepted.
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// GitHubOIDC authenticates callers via GitHub Actions OIDC tokens.
	// No secret is required — the gateway verifies the JWT signature against
	// GitHub's public JWKS and matches the claims in the allow block.
	GitHubOIDC *GitHubOIDC `yaml:"githubOIDC,omitempty" json:"githubOIDC,omitempty"`

	// GitLabOIDC authenticates callers via GitLab CI OIDC tokens.
	GitLabOIDC *GitLabOIDC `yaml:"gitlabOIDC,omitempty" json:"gitlabOIDC,omitempty"`

	// VaultOIDC authenticates callers via HashiCorp Vault OIDC tokens.
	// The caller authenticates to Vault first (via any Vault auth method), then
	// presents a Vault-issued OIDC token to the gateway. No stored secret needed.
	// The gateway discovers the JWKS via {url}/v1/identity/oidc/.well-known/openid-configuration.
	VaultOIDC *VaultOIDC `yaml:"vaultOIDC,omitempty" json:"vaultOIDC,omitempty"`

	// OIDC authenticates callers via any OIDC-compliant identity provider.
	// Issuer is required; the gateway discovers the JWKS URI via
	// {issuer}/.well-known/openid-configuration.
	OIDC *OIDCToken `yaml:"oidc,omitempty" json:"oidc,omitempty"`
}

// APISecretRef locates a Kubernetes Secret that holds a bearer token.
type APISecretRef struct {
	// Name is the Kubernetes Secret name.
	Name string `yaml:"name" json:"name"`

	// Key is the data key within the Secret whose value is the token.
	Key string `yaml:"key" json:"key"`

	// Namespace is the Secret's namespace.
	// Defaults to Inrun's own namespace when empty.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// RotateAfter is an optional duration (e.g. "90d", "720h").
	// When set, the gateway checks the inrun.dev/generated-at annotation on
	// the Secret; if the age exceeds this duration, it deletes and recreates
	// the Secret with a new uuidv4 token.
	RotateAfter string `yaml:"rotateAfter,omitempty" json:"rotateAfter,omitempty"`
}

// ── GatewayConfig methods ──────────────────────────────────────────

// HasAPI reports whether the Gateway API is enabled and configured.
func (g *GatewayConfig) HasAPI() bool {
	if g == nil {
		return false
	}
	if g.API == nil {
		return false
	}
	return g.API.Enabled
}

// HasClusters reports whether any remote clusters are registered.
func (g *GatewayConfig) HasClusters() bool {
	return g != nil && g.Clusters != nil && len(g.Clusters.Entries) > 0
}

// ClusterNames returns the list of registered cluster names.
func (g *GatewayConfig) ClusterNames() []string {
	if !g.HasClusters() {
		return nil
	}
	names := make([]string, 0, len(g.Clusters.Entries))
	for name := range g.Clusters.Entries {
		names = append(names, name)
	}
	return names
}

// Cluster returns the config for a named cluster and whether it exists.
func (g *GatewayConfig) Cluster(name string) (GatewayClusterConfig, bool) {
	if !g.HasClusters() {
		return GatewayClusterConfig{}, false
	}
	cfg, ok := g.Clusters.Entries[name]
	return cfg, ok
}

// ── GatewayClusterConfig methods ──────────────────────────────

// HasSecretRef reports whether the kubeconfig credential form is declared.
func (c GatewayClusterConfig) HasSecretRef() bool {
	return c.SecretRef != nil
}

// HasTokenRef reports whether the bearer-token credential form is declared.
func (c GatewayClusterConfig) HasTokenRef() bool {
	return c.TokenRef != nil
}

// HasCARef reports whether a CA cert ref is declared.
func (c GatewayClusterConfig) HasCARef() bool {
	return c.CARef != nil
}

// CredentialForm returns a string identifying which credential form is set:
// "kubeconfig", "token", or "" (none declared).
func (c GatewayClusterConfig) CredentialForm() string {
	if c.HasSecretRef() {
		return "kubeconfig"
	}
	if c.HasTokenRef() || c.HasCARef() {
		return "token"
	}
	return ""
}

// HasCredentials reports whether any credential form is declared.
func (c GatewayClusterConfig) HasCredentials() bool {
	return c.CredentialForm() != ""
}

// EndpointURL returns the API server URL for this cluster.
func (c GatewayClusterConfig) EndpointURL() string { return c.Endpoint }

// IsInsecure reports whether TLS verification is skipped for this cluster.
func (c GatewayClusterConfig) IsInsecure() bool { return c.Insecure }

// ── APISecretRef methods ──────────────────────────────────────

// SecretName returns the Kubernetes Secret name.
func (r *APISecretRef) SecretName() string {
	if r == nil {
		return ""
	}
	return r.Name
}

// SecretKey returns the data key within the Secret.
func (r *APISecretRef) SecretKey() string {
	if r == nil {
		return ""
	}
	return r.Key
}

// SecretNamespace returns the Secret namespace, or empty string to use the default.
func (r *APISecretRef) SecretNamespace() string {
	if r == nil {
		return ""
	}
	return r.Namespace
}

// IsValid reports whether the ref is structurally complete: name and key are required.
// Namespace is optional — an empty namespace means "use the operator's own namespace".
func (r *APISecretRef) IsValid() bool {
	if r == nil {
		return false
	}
	return strings.TrimSpace(r.Name) != "" && strings.TrimSpace(r.Key) != ""
}

// ── APIConfig methods ─────────────────────────────────────────

// HasAuth reports whether the Gateway API has authentication configured.
func (a *GatewayAPIConfig) HasAuth() bool {
	if a == nil {
		return false
	}
	return a.Auth.HasTokens()
}

// ── APIAuth methods ───────────────────────────────────────────

// HasTokens reports whether at least one token is configured.
func (a APIAuth) HasTokens() bool {
	return len(a.Tokens) > 0
}

// Empty reports whether the auth struct is completely unconfigured.
func (a APIAuth) Empty() bool {
	return len(a.Tokens) == 0
}

// BlueprintLifecyclePolicy holds lifecycle-related enforcement rules within a policy: block.
type BlueprintLifecyclePolicy struct {
	// MinMaturity sets the minimum lifecycle maturity allowed for imported patterns.
	// Imports below this floor are errors at inrun validate time rather than warnings.
	// Valid values: alpha, beta, stable (deprecated imports are always errors without accept).
	MinMaturity LifecycleMaturity `yaml:"minMaturity,omitempty" json:"minMaturity,omitempty"`
}

// BlueprintPolicy declares platform-level enforcement rules for a Stack.
// Policy is distinct from lifecycle: — it is a platform-tier concern that
// governs what imports are allowed, not what the pattern itself signals.
// Structured as policy.<area>.* so new policy categories (security, registry,
// user-defined) can grow alongside lifecycle without flattening into one block.
type BlueprintPolicy struct {
	Lifecycle *BlueprintLifecyclePolicy `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"`
}

// BlueprintFile is the top-level structure of a blueprint.yaml file.
// It contains optional imports (files and helm charts) plus inline CRDs.
// Inrun's in-built merger resolves all imports and merges everything into one BlueprintSpec.
type BlueprintFile struct {
	APIVersion string              `yaml:"apiVersion"`
	Kind       string              `yaml:"kind"`
	Metadata   BlueprintMeta       `yaml:"metadata"`
	Lifecycle  *BlueprintLifecycle `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"`
	Policy     *BlueprintPolicy    `yaml:"policy,omitempty"    json:"policy,omitempty"`
	Imports    *BlueprintSources   `yaml:"imports,omitempty"`
	Spec       BlueprintSpec       `yaml:"spec"`
	Security   BlueprintSecurity   `yaml:"security"`

	// CrossAccess sets the default cross-read policy for all CRDs in this Blueprint.
	// When false, no other Blueprint may read any CRD in this one via cross:.
	// Individual CRDs may override with their own crossAccess field.
	// Defaults to true (open) when not declared.
	CrossAccess *bool `yaml:"crossAccess,omitempty" json:"crossAccess,omitempty"`

	// Gateway declares how the gateway is deployed for this Blueprint.
	// When gateway.standalone: true, the gateway runs without a runtime operator
	// and spec: may be empty.
	Gateway *GatewayConfig `yaml:"gateway,omitempty" json:"gateway,omitempty"`

	// Publish declares the publishing and consumer policy for this pattern.
	// Controls signing requirements and which quality gates run at push time.
	// Distinct from security: — publish: is about supply chain, not runtime admission.
	Publish *PublishConfig `yaml:"publish,omitempty" json:"publish,omitempty"`

	// Functions declares user-defined functions available to all CRDs in this Blueprint.
	// Functions are named template expressions that compose built-in functions and Go template
	// syntax. Once declared, a function is callable by name in any template expression.
	Functions KELFunctionRegistry `yaml:"functions,omitempty"`

	// Presets declares named presets available to all CRDs in this Blueprint.
	// Presets are resolved before built-in Inrun presets at both validate
	// and reconcile time. Template expressions in preset field values are
	// resolved at reconcile time; validation skips fields that contain {{ }}.
	Presets PresetRegistry `yaml:"presets,omitempty"`
}

// LooksLikeStack reports if this document looks like a Stack
func (c *BlueprintFile) LooksLikeStack() bool {
	if c == nil {
		return false
	}
	return c.Imports != nil && (len(c.Imports.Files) > 0 || len(c.Imports.Registry) > 0 || len(c.Imports.Helm) > 0)
}

// WithImportsOrSpecOverrides reports if this document should be a Stack (has imports or inline CRDs)
func (c *BlueprintFile) WithImportsOrSpecOverrides() bool {
	if c == nil {
		return false
	}
	return c.Imports != nil || len(c.Spec.CRDs) > 0
}

// BlueprintMeta holds identifying metadata for a Blueprint.
type BlueprintMeta struct {
	// Name is the required unique identifier of the Blueprint.
	Name string `yaml:"name" json:"name,omitempty"`

	// Namespace scopes this Blueprint to a logical tenant or team within a single
	// runtime. Defaults to "default" when not declared — identical to Kubernetes
	// namespace semantics. The Console groups CRDs by namespace so each
	// team sees only its own panel.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// ClusterName identifies the cluster this Blueprint runs in.
	// Used by the Console for cluster-level filtering when multiple
	// runtimes are connected. Blueprint value takes precedence over the
	// CLUSTER_NAME env var. Empty when neither is set.
	ClusterName string `yaml:"clusterName,omitempty" json:"clusterName,omitempty"`

	// Description provides a human-readable explanation of the Blueprint's purpose.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Version follows semantic versioning (e.g., "1.2.3") for the Blueprint schema or content.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`

	// Author identifies the creator or maintainer of the Blueprint.
	Author string `yaml:"author,omitempty" json:"author,omitempty"`

	// License describes the licensing terms under which the Blueprint is distributed.
	License string `yaml:"license,omitempty" json:"license,omitempty"`

	// Tags are optional keywords for categorising the Blueprint in the Inrun Registry.
	// They aid discovery (e.g., "database", "stateful", "security") when using
	// `inrun patterns --tag <tag>` and for indexing in Artifact Hub.
	// Tags have no effect on runtime behaviour.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// DeprecationTimeline sets the date window for deprecation display.
// Both fields are YYYY-MM-DD strings.
type DeprecationTimeline struct {
	From string `yaml:"from,omitempty" json:"from,omitempty"` // warn from this date
	To   string `yaml:"to,omitempty"   json:"to,omitempty"`   // EOL on this date
}

// LifecycleMaturity signals the stability level of a Blueprint pattern.
type LifecycleMaturity string

const (
	MaturityAlpha      LifecycleMaturity = "alpha"
	MaturityBeta       LifecycleMaturity = "beta"
	MaturityStable     LifecycleMaturity = "stable"
	MaturityDeprecated LifecycleMaturity = "deprecated"
)

// LifecycleCompat declares which Kubernetes and Inrun versions this pattern
// has been verified against. Both fields accept Masterminds semver range syntax.
type LifecycleCompat struct {
	Kubernetes string `yaml:"kubernetes,omitempty" json:"kubernetes,omitempty"`
	Inrun      string `yaml:"inrun,omitempty"   json:"inrun,omitempty"`
}

// StackAcceptEntry acknowledges the lifecycle state of a single imported pattern.
// Naming a pattern here accepts any deprecation or pre-stable maturity concern for
// that import. Version, when set, scopes the acceptance to a semver range — inrun
// validate warns when the imported version no longer matches (stale acceptance).
type StackAcceptEntry struct {
	Name    string `yaml:"name"`
	Author  string `yaml:"author,omitempty"  json:"author,omitempty"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"` // semver range; omit = all versions
}

// StackAccept is valid only on a Stack. It declares which imported patterns
// the Stack author has evaluated and accepted, regardless of their lifecycle state.
type StackAccept struct {
	Patterns []StackAcceptEntry `yaml:"patterns,omitempty" json:"patterns,omitempty"`
}

// Accepts reports whether the given pattern name (and optional author) is covered.
func (a *StackAccept) Accepts(name, author string) bool {
	if a == nil {
		return false
	}
	for _, e := range a.Patterns {
		if e.Name != name {
			continue
		}
		if author != "" && e.Author != "" && e.Author != author {
			continue
		}
		return true
	}
	return false
}

// BlueprintLifecycle is the top-level policy block for a Blueprint. It governs
// maturity signals, deprecation, and compatibility gates. The CLI reads it:
// inrun validate, push and inspect show it, and inrun and inrun gate run refuse
// to start a deprecated Blueprint unless a Stack accepts it
// (CheckDeprecationPolicy). The runtime in a cluster does not read it.
type BlueprintLifecycle struct {
	Maturity      LifecycleMaturity     `yaml:"maturity,omitempty"      json:"maturity,omitempty"`
	Deprecation   *BlueprintDeprecation `yaml:"deprecation,omitempty"   json:"deprecation,omitempty"`
	Compatibility *LifecycleCompat      `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
	// Accept is only valid on a Stack. It acknowledges lifecycle concerns of imported patterns.
	Accept *StackAccept `yaml:"accept,omitempty" json:"accept,omitempty"`
}

// IsDeprecated reports whether the lifecycle block declares the pattern deprecated.
func (l *BlueprintLifecycle) IsDeprecated() bool {
	if l == nil {
		return false
	}
	return l.Maturity == MaturityDeprecated || (l.Deprecation != nil && l.Deprecation.IsDeprecated())
}

// BlueprintDeprecation carries deprecation guidance for registry consumers.
type BlueprintDeprecation struct {
	Timeline   *DeprecationTimeline `yaml:"timeline,omitempty"   json:"timeline,omitempty"`
	MigratedTo string               `yaml:"migratedTo,omitempty" json:"migratedTo,omitempty"`
	Message    string               `yaml:"message,omitempty"    json:"message,omitempty"`
}

// IsDeprecated indicates that this blueprint is deprecated and should surface
// warnings in inrun validate, inrun inspect, and registry consumers.
func (d *BlueprintDeprecation) IsDeprecated() bool {
	if d == nil {
		return false
	}
	return d.MigratedTo != "" || d.Message != "" || d.Timeline != nil
}

// MigrationTarget returns the value of the MigratedTo field.
// If the deprecation block is nil or empty, it returns an empty string.
func (d *BlueprintDeprecation) MigrationTarget() string {
	if d == nil {
		return ""
	}
	return d.MigratedTo
}

// MigrationMessage returns the deprecation message.
// If the deprecation block is nil or empty, it returns an empty string.
func (d *BlueprintDeprecation) MigrationMessage() string {
	if d == nil {
		return ""
	}
	return d.Message
}

// DeprecationState returns "none", "warning", or "eol" based on today vs the
// timeline. Returns "warning" when no timeline is set and the block is present
// (legacy behaviour — always show warning if deprecated is declared).
func (d *BlueprintDeprecation) DeprecationState(today time.Time) string {
	if d == nil {
		return "none"
	}
	t := d.Timeline
	if t == nil {
		if d.IsDeprecated() {
			return "warning"
		}
		return "none"
	}
	const layout = "2006-01-02"
	todayDate := today.Truncate(24 * time.Hour)
	if t.From != "" {
		from, err := time.Parse(layout, t.From)
		if err == nil && todayDate.Before(from) {
			return "none"
		}
	}
	if t.To != "" {
		to, err := time.Parse(layout, t.To)
		if err == nil && !todayDate.Before(to) {
			return "eol"
		}
	}
	return "warning"
}

// TimelineFrom returns the timeline.from date string, or empty if not set.
func (d *BlueprintDeprecation) TimelineFrom() string {
	if d == nil || d.Timeline == nil {
		return ""
	}
	return d.Timeline.From
}

// TimelineTo returns the timeline.to date string, or empty if not set.
func (d *BlueprintDeprecation) TimelineTo() string {
	if d == nil || d.Timeline == nil {
		return ""
	}
	return d.Timeline.To
}

// HasTimeline reports whether a timeline block is present with at least one date.
func (d *BlueprintDeprecation) HasTimeline() bool {
	if d == nil || d.Timeline == nil {
		return false
	}
	return d.Timeline.From != "" || d.Timeline.To != ""
}

// DaysUntilEOL returns how many days remain until timeline.to, or -1 if not set.
func (d *BlueprintDeprecation) DaysUntilEOL(today time.Time) int {
	if d == nil || d.Timeline == nil || d.Timeline.To == "" {
		return -1
	}
	to, err := time.Parse("2006-01-02", d.Timeline.To)
	if err != nil {
		return -1
	}
	days := int(to.Truncate(24*time.Hour).Sub(today.Truncate(24*time.Hour)).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// BlueprintSources declares where to load CRD definitions from.
// Sources are loaded before spec.crds — inline CRDs are merged last
// and win on name conflict (allowing local overrides of remote definitions).
//
// Only valid on kind: Stack documents.
type BlueprintSources struct {
	// Files — local paths, remote URLs, or environment variable references.
	// Each entry must be a valid Blueprint YAML (apiVersion, kind, spec.crds).
	// Supports environment variable references: $MY_BLUEPRINT_URL
	//
	// Simple form: just a path string (no auth)
	//   files:
	//     - ./blueprints/project.yaml
	//     - https://public.url/blueprint.yaml
	//     - $MY_BLUEPRINT_URL
	//
	// Authenticated form: a FileSource struct with auth block
	//   files:
	//     - url: https://private.url/blueprint.yaml
	//       auth:
	//         type: bearer
	//         fromEnv: MY_TOKEN
	Files []FileSource `yaml:"files,omitempty"`

	// Helm — Helm chart sources. Each chart is rendered with the provided
	// value files and the resulting Blueprint templates are extracted and merged.
	Helm []HelmSource `yaml:"helm,omitempty"`

	// Registry - Registry sources.
	Registry []RegistrySource `yaml:"registry,omitempty"`
}

// HelmSource declares a Helm chart that produces Blueprint CRD definitions.
// The chart must render at least one template with kind: Blueprint.
//
// Example chart template (templates/blueprint.yaml):
//
//	apiVersion: inrun.dev/v1
//	kind: Blueprint
//	spec:
//	  crds:
//	    {{- range .Values.crds }}
//	    - name: {{ .name }}
//	      enabled: {{ .enabled }}
//	      ...
//	    {{- end }}
type HelmSource struct {
	// Repo — Helm repository URL.
	// e.g. "https://charts.myorg.io"
	Repo string `yaml:"repo" validate:"required"`

	// Chart — chart name within the repository.
	// e.g. "platform-crds"
	Chart string `yaml:"chart" validate:"required"`

	// Version — chart version to use. Required for reproducibility.
	// And also used as git ref
	// e.g. "1.2.0"
	Version string `yaml:"version" validate:"required"`

	// Path — chart path within git repo
	Path string `yaml:"path"       validate:"omitempty"`

	// ValueFiles — list of values files to apply when rendering the chart.
	// Each entry can be a local path or a remote URL.
	// Supports environment variable references: $MY_VALUES_FILE
	// Applied in order — later files override earlier ones (same as helm -f).
	ValueFiles []string `yaml:"valueFiles,omitempty"`

	// Values — inline key-value pairs applied after valueFiles.
	// Same as helm --set key=value.
	Values map[string]interface{} `yaml:"values,omitempty"`
}

// BlueprintSpec holds the actual CRD definitions.
// This is what the merger produces after resolving all sources.
type BlueprintSpec struct {
	// Imports declares Modules whose presets are merged into the Blueprint-wide
	// PresetRegistry. Only presets: from each Module are consumed here;
	// resources, status, and admission declarations in the Module are ignored
	// at this level. Use spec.crds[name].imports for those.
	Imports []ModuleImport `yaml:"imports,omitempty"`

	// Finalizers — Blueprint-level finalizers applied to all CRDs
	// unless overridden at the CRD level.
	Finalizers []string `yaml:"finalizers,omitempty"`

	// CRDs — the CRD entries managed by this Inrun instance.
	// Map key is the CRD name; Name field is injected from the key during loading.
	CRDs map[string]CRDEntry `yaml:"crds"`
}

// BlueprintForUI is a UI-friendly representation of the merged Blueprint.
// It contains only the fields needed for display in the Console,
// excluding internal runtime fields.
type BlueprintForUI struct {
	APIVersion string             `json:"apiVersion"` // Inrun API version
	Kind       string             `json:"kind"`       // Always "Blueprint" at runtime
	Metadata   BlueprintMeta      `json:"metadata"`   // Blueprint metadata (name, description, etc.)
	Spec       BlueprintSpecForUI `json:"spec"`       // CRD definitions
	Security   BlueprintSecurity  `json:"security"`   // Security settings
}

// BlueprintSpecForUI contains the CRD definitions for UI display.
type BlueprintSpecForUI struct {
	CRDs map[string]CRDEntry `json:"crds"` // Map of CRD name to CRD definition
}
