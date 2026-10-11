// pkg/types/types_reconcile_config.go
package types

import "github.com/intentd/inrun/domain"

// RemoteReconcileProtocol is the wire protocol used for a remote reconciler call.
type RemoteReconcileProtocol string

const (
	RemoteReconcileProtocolHTTP RemoteReconcileProtocol = "http"
	RemoteReconcileProtocolGRPC RemoteReconcileProtocol = "grpc" // reserved
)

// String returns the string representation of a RemoteReconcileProtocol.
func (r RemoteReconcileProtocol) String() string { return string(r) }

// ValidRemoteReconcileProtocols returns all known RemoteReconcileProtocol values.
func ValidRemoteReconcileProtocols() []string {
	return []string{string(RemoteReconcileProtocolHTTP), string(RemoteReconcileProtocolGRPC)}
}

// IsValidRemoteReconcileProtocol reports whether s is a known RemoteReconcileProtocol.
func IsValidRemoteReconcileProtocol(s string) bool {
	switch RemoteReconcileProtocol(s) {
	case RemoteReconcileProtocolHTTP, RemoteReconcileProtocolGRPC:
		return true
	}
	return false
}

// RemoteReconcilerDeclaration declares a remote HTTP reconciler.
// The coordinator POSTs a PreparedRequest to Endpoint; the response is a RemoteReconcileResult.
// Inrun owns the queue, backoff, informer, health, and metrics. The remote service owns logic.
type RemoteReconcilerDeclaration struct {
	// Protocol is the wire protocol. Default http; grpc reserved.
	Protocol RemoteReconcileProtocol `yaml:"protocol,omitempty" json:"protocol,omitempty"`

	// Endpoint is the URL the PreparedRequest is POSTed to. Required.
	// Template expressions are supported and evaluated per-reconcile against the CR's resolver context:
	// e.g. "http://{{ .metadata.namespace }}-svc.svc.cluster.local/reconcile"
	Endpoint string `yaml:"endpoint" json:"endpoint" validate:"required"`

	// Auth declares the credential for the outbound request. Uses the same shape as ExternalCallSpec.
	Auth *ExternalAuth `yaml:"auth,omitempty" json:"auth,omitempty"`

	// Timeout caps the round-trip. Defaults to 30s when omitted.
	Timeout Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Request controls what the outbound request body carries.
	Request *RemoteRequestConfig `yaml:"request,omitempty" json:"request,omitempty"`
}

// RemoteRequestConfig controls what is included in the outbound PreparedRequest body.
type RemoteRequestConfig struct {
	// Object controls which paths are stripped from the CR object before serialisation.
	Object *RemoteRequestObjectConfig `yaml:"object,omitempty" json:"object,omitempty"`

	// Children controls which previously-created child resources are injected into the request.
	Children *RemoteRequestChildrenConfig `yaml:"children,omitempty" json:"children,omitempty"`
}

// RemoteRequestObjectConfig strips paths from the CR object before it is sent to the reconciler.
type RemoteRequestObjectConfig struct {
	// Exclude is a list of dot-notation paths to remove from the object.
	// Example: ["metadata.managedFields", "metadata.annotations"]
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

// RemoteRequestChildrenConfig controls which child resources are injected into prepared.children.
type RemoteRequestChildrenConfig struct {
	// Enabled controls whether children are injected at all.
	// Default when omitted: true. Set to false to disable children injection entirely,
	// useful for reconcilers that never create Kubernetes resources.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Resources maps each allowed resource type name to its per-type config.
	// When omitted or nil: all types declared in managedResources are injected.
	// When set: only the named types are included; each entry may override the root Exclude list.
	// Example:
	//   resources:
	//     deployment:
	//       exclude: ["spec.template.metadata.annotations"]
	//     service: {}
	Resources map[string]*RemoteRequestChildrenResourceConfig `yaml:"resources,omitempty" json:"resources,omitempty"`

	// Exclude is a list of dot-notation paths to strip from every injected child object.
	// Per-resource exclude (in resources.<type>.exclude) takes precedence for that type.
	// Example: ["metadata.managedFields"]
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

// RemoteRequestChildrenResourceConfig is the per-resource-type config inside request.children.resources.
type RemoteRequestChildrenResourceConfig struct {
	// Exclude overrides the root exclude list for this resource type.
	// When nil: the root Exclude applies. When empty slice: no paths are stripped for this type.
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

// HasResources reports whether an explicit resources allow-list is declared.
func (c *RemoteRequestChildrenConfig) HasResources() bool {
	return c != nil && len(c.Resources) > 0
}

// HasExclude reports whether a root exclude list is declared.
func (c *RemoteRequestChildrenConfig) HasExclude() bool {
	return c != nil && len(c.Exclude) > 0
}

// IsEnabled reports whether children injection is active.
// Defaults to true when Enabled is nil.
func (c *RemoteRequestChildrenConfig) IsEnabled() bool {
	return c == nil || c.Enabled == nil || *c.Enabled
}

// EffectiveChildren returns the active children config for callers to use.
// Returns nil only when injection is explicitly disabled (enabled: false).
// Returns an empty config (inject all, no filtering) when no config is declared.
// All callers must go through this rather than accessing Children directly.
func (p *RemoteRequestConfig) EffectiveChildren() *RemoteRequestChildrenConfig {
	if p == nil || p.Children == nil {
		return &RemoteRequestChildrenConfig{} // default: inject all
	}
	if !p.Children.IsEnabled() {
		return nil // explicitly disabled
	}
	return p.Children
}

// ChildrenEnabled reports whether the request carries the CR's children: true
// unless request.children.enabled is false.
func (p *RemoteRequestConfig) ChildrenEnabled() bool {
	return p.EffectiveChildren() != nil
}

// ReconcileConfig declares what runs and how. Default (omitted or default: true) uses the
// generic.Reconciler driven by onCreate/onReconcile/onDelete templates. Set default: false
// and declare constructor: to bring a typed Go reconciler. Workers, resync, queue, and
// requeue tune execution regardless of which reconciler runs.
type ReconcileConfig struct {
	// --- Implementation identity ---

	// Default: true → generic.Reconciler (default when omitted).
	// false → custom reconciler via Constructor.
	Default *bool `yaml:"default,omitempty" json:"default,omitempty" validate:"omitempty"`

	// Hooks declares a Go hook function. Signature: func() domain.AnyReconcileHooks.
	Hooks *HookDeclaration `yaml:"hooks,omitempty" json:"hooks,omitempty" validate:"omitempty"`

	// Constructor declares a custom reconciler constructor. Required when Default: false.
	ConstructorDecl *ConstructorDeclaration `yaml:"constructor,omitempty" json:"constructor,omitempty" validate:"omitempty"`

	// Remote declares a remote HTTP reconciler. Required when Default: false and no constructor is set.
	Remote *RemoteReconcilerDeclaration `yaml:"remote,omitempty" json:"remote,omitempty" validate:"omitempty"`

	// --- Execution parameters ---

	// Profile is a named tuning preset. Built-ins: high-throughput, conservative, development.
	// Inline Workers/Resync/Queue override the profile.
	Profile string `yaml:"profile,omitempty" json:"profile,omitempty"`

	// Workers is the number of concurrent reconcile goroutines. 0 → DEFAULT_WORKERS.
	Workers int `yaml:"workers,omitempty" json:"workers,omitempty" validate:"omitempty,gte=1,lte=50"`

	// Resync is the full re-list interval for the informer cache. 0 → DEFAULT_RESYNC.
	Resync Duration `yaml:"resync,omitempty" json:"resync,omitempty"`

	Queue   Queue          `yaml:"queue,omitempty" json:"queue,omitempty"`
	Requeue *RequeueConfig `yaml:"requeue,omitempty" json:"requeue,omitempty"`

	// --- Declarative lifecycle ---

	// Normalize normalizes declared spec fields before template rendering.
	Normalize *NormalizeConfig `yaml:"normalize,omitempty" json:"normalize,omitempty"`

	// Imports declares Module imports merged into onReconcile at load time.
	Imports []ModuleImport `yaml:"imports,omitempty" json:"imports,omitempty"`

	// ForceConflict sets Force: true on server-side apply for child resources.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`

	// Args are given to the reconciler on every reconcile as Request.Args.
	// Template expressions are rendered against the CR, so args express the CR
	// in the reconciler's own vocabulary; the mapping is free. Used by hooks,
	// constructor and remote reconcilers; rejected on declarative-only CRDs.
	Args map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`

	// External declares HTTP calls the runtime makes before each reconcile.
	// Results are available as .external.<name>.* in reconcile.args templates.
	// Used by hooks, constructor and remote reconcilers; declarative templates
	// declare their own external calls under onCreate / onReconcile.
	External []ExternalCallSpec `yaml:"external,omitempty" json:"external,omitempty"`

	// ManagedResources lists the resource types the reconciler may create: what
	// a Result's resources are checked against, and what Inrun generates RBAC
	// for. Used by hooks, constructor and remote reconcilers; rejected on
	// declarative-only CRDs (their RBAC comes from their templates).
	ManagedResources []domain.ManagedResource `yaml:"managedResources,omitempty" json:"managedResources,omitempty"`

	OnCreate    *HookTemplates `yaml:"onCreate,omitempty" json:"onCreate,omitempty" validate:"omitempty"`
	OnReconcile *HookTemplates `yaml:"onReconcile,omitempty" json:"onReconcile,omitempty" validate:"omitempty"`
	OnDelete    *HookTemplates `yaml:"onDelete,omitempty" json:"onDelete,omitempty" validate:"omitempty"`

	// --- At runtime mapping ---

	HookFactory func() domain.AnyReconcileHooks `yaml:"-" json:"-"`
	Constructor NewReconcilerFunc               `yaml:"-" json:"-"`

	// Include is a path to a YAML file whose reconcile: block is merged under this config.
	// Inline fields take precedence. Cleared after expansion.
	Include string `yaml:"include,omitempty" json:"include,omitempty"`
}

func (r *ReconcileConfig) Empty() bool { return r == nil }

// IsDefault reports whether the generic.Reconciler should be used.
func (r *ReconcileConfig) IsDefault() bool {
	if r == nil || r.Default == nil {
		return true
	}
	return *r.Default
}

// HasHooksDecl reports whether a hook declaration exists.
func (r *ReconcileConfig) HasHooksDecl() bool {
	return r != nil && r.Hooks != nil && r.Hooks.Location != ""
}

// HasConstructorDecl reports whether a constructor declaration exists.
func (r *ReconcileConfig) HasConstructorDecl() bool {
	return r != nil && r.ConstructorDecl != nil && r.ConstructorDecl.Location != ""
}

// HasRemoteDecl reports whether a remote reconciler declaration exists.
func (r *ReconcileConfig) HasRemoteDecl() bool {
	return r != nil && r.Remote != nil
}

// HasRetryBackoff reports whether a retryBackoff is declared on this reconcile config's queue.
func (r *ReconcileConfig) HasRetryBackoff() bool {
	return r != nil && r.Queue.HasRetryBackoff()
}

// IsRequeueEmpty reports whether the requeue configuration is effectively empty.
func (r *ReconcileConfig) IsRequeueEmpty() bool {
	if r == nil || r.Requeue == nil {
		return true
	}
	rc := r.Requeue
	return rc.After == "" && len(rc.When) == 0 && len(rc.Or) == 0
}
