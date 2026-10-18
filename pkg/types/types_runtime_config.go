// pkg/types/types_runtime_config.go
package types

import (
	"time"

	"github.com/intentd/kel/core"
)

// RuntimeConfig governs the operatorBox as a long-lived entity, not a single reconcile cycle.
// Covers autoscaling the worker pool, finalizer lifecycle, namespace guards, and deletion protection.
type RuntimeConfig struct {
	// Finalizers configures the finalizers on this CRD's CRs: extra names, a
	// finalizing timeout, and removal. Inrun's own inrun.dev/cleanup is always
	// added and never listed.
	Finalizers *FinalizerConfig `yaml:"finalizers,omitempty" json:"finalizers,omitempty" validate:"omitempty"`

	// DeletionProtection overrides the global deletion protection policy for this CRD.
	DeletionProtection *DeletionProtectionOverride `yaml:"deletionProtection,omitempty" json:"deletionProtection,omitempty"`

	// RestrictedNamespaces blocks reconciliation for CRs in the named namespaces.
	RestrictedNamespaces RestrictedNamespaces `yaml:"restrictedNamespaces,omitempty" json:"restrictedNamespaces,omitempty"`

	// AllowedNamespaces restricts reconciliation to CRs in the named namespaces only.
	AllowedNamespaces AllowedNamespaces `yaml:"allowedNamespaces,omitempty" json:"allowedNamespaces,omitempty"`

	// IgnoreStatusPatch disables the runtime's automatic status patch for this CRD.
	IgnoreStatusPatch bool `yaml:"ignoreStatusPatch,omitempty" json:"ignoreStatusPatch,omitempty"`

	// IgnoreObservedGeneration disables generation-based reconcile skipping for this CRD.
	IgnoreObservedGeneration bool `yaml:"ignoreObservedGeneration,omitempty" json:"ignoreObservedGeneration,omitempty"`

	// Autoscale declares runtime worker/queue/resync overrides driven by conditions.
	Autoscale *AutoscaleSpec `yaml:"autoscale,omitempty" json:"autoscale,omitempty"`

	// Cleanup declares when to delete the CR after it reaches a terminal state.
	// Conditions are evaluated before each reconcile; once met, the CR is deleted
	// when after elapses. The reconciler keeps running during the grace period.
	// Works for all reconciler types: remote, generic, and typed. Reconcilers can
	// also request cleanup themselves through Result.Cleanup.
	Cleanup *CleanupConfig `yaml:"cleanup,omitempty" json:"cleanup,omitempty"`
}

// CleanupConfig declares when to delete a CR.
// When declares AND conditions; Or declares OR conditions. Both must pass when both
// are declared (same semantics as GateConditions). The most common pattern is
// Or-only: delete when phase is Completed OR Failed.
type CleanupConfig struct {
	// When declares AND conditions — all must be true.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or declares OR conditions — at least one must be true.
	// When both When and Or are declared, both must pass.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// After is an optional grace period between the conditions being met and
	// the deletion. Inrun annotates the CR with the first-met time, so the clock
	// survives restarts. Zero means delete immediately.
	After Duration `yaml:"after,omitempty" json:"after,omitempty"`
}

// HasCleanup reports whether a cleanup block with at least one condition is declared.
func (r *RuntimeConfig) HasCleanup() bool {
	return r != nil && r.Cleanup != nil && (len(r.Cleanup.When) > 0 || len(r.Cleanup.Or) > 0)
}

// EffectiveCleanup returns the cleanup config or nil when absent.
func (r *RuntimeConfig) EffectiveCleanup() *CleanupConfig {
	if r == nil {
		return nil
	}
	return r.Cleanup
}

func (r *RuntimeConfig) Empty() bool { return r == nil }

// FinalizerConfig is runtime.finalizers.
type FinalizerConfig struct {
	// Names are finalizers added to every CR besides Inrun's inrun.dev/cleanup.
	// The Catalog-level spec.finalizers are merged in.
	Names []string `yaml:"names,omitempty" json:"names,omitempty"`

	// Timeout releases a CR still finalizing this long after its deletion
	// started: Inrun removes its finalizers even though the reconciler has not
	// returned without a requeue, and emits a warning. A duration such as "30m"
	// or "2h"; off unless set. Read it with TimeoutDuration.
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Remove strips the finalizers from this CRD's CRs instead of adding them.
	// Testing only.
	Remove bool `yaml:"remove,omitempty" json:"remove,omitempty"`
}

// SetFinalizerNames sets runtime.finalizers.names, creating the block when absent.
func (r *RuntimeConfig) SetFinalizerNames(names []string) {
	if r.Finalizers == nil {
		r.Finalizers = &FinalizerConfig{}
	}
	r.Finalizers.Names = names
}

// TimeoutDuration parses Timeout; 0 when unset. Validation rejects anything
// that does not parse to a positive duration, so errors are not expected here.
func (f *FinalizerConfig) TimeoutDuration() time.Duration {
	if f == nil || f.Timeout == "" {
		return 0
	}
	d, err := core.ParseTimeDuration(f.Timeout)
	if err != nil {
		return 0
	}
	return d
}
