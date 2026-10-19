package types

import (
	"github.com/intentd/inrun/pkg/util/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// MaxServeTargetFieldSelector is the maximum number of field selectors per target.
	MaxServeTargetFieldSelector = 3
)

// ServeApplyOverrides configures override behaviours for apply operations.
type ServeApplyOverrides struct {
	// TargetConflict, when true, allows callers to change the target/alias
	// of an existing CR via the ?override=true query parameter.
	// Default: false — target/alias is immutable after creation.
	TargetConflict *bool `yaml:"targetConflict,omitempty" json:"targetConflict,omitempty"`

	// ResourceConflict, when true, sets Force: true on every server-side apply
	// for this CRD — the gateway takes ownership of any conflicting fields
	// rather than surfacing a conflict error. Equivalent to helm --force-conflict.
	// Can be overridden per-request with ?overwrite=true regardless of this setting.
	// Default: false.
	ResourceConflict *bool `yaml:"resourceConflict,omitempty" json:"resourceConflict,omitempty"`

	// KeepPreviousSurface, when true, leaves resources created by a prior serve
	// target alive when the CR switches to a new target. By default (false) the
	// reconciler deletes orphaned child resources whose serve-target label no
	// longer matches the CR's active surface.
	KeepPreviousSurface *bool `yaml:"keepPreviousSurface,omitempty" json:"keepPreviousSurface,omitempty"`
}

// ServeApplyConfig configures apply-time behaviour for a CRD or target.
type ServeApplyConfig struct {
	// Overrides contains override settings for apply operations.
	Overrides *ServeApplyOverrides `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

// ─── HasOverride methods ─────────────────────────────────────────────────────

// HasOverride reports whether the ServeConfig has any override fields set.
// Used to avoid unnecessary config blocks in the Blueprint.
func (s *ServeConfig) HasOverride() bool {
	if s == nil || s.Apply == nil || s.Apply.Overrides == nil {
		return false
	}
	o := s.Apply.Overrides
	return o.TargetConflict != nil || o.ResourceConflict != nil || o.KeepPreviousSurface != nil
}

// HasOverride reports whether the ServeTargetConfig has any override fields set.
func (t *ServeTargetConfig) HasOverride() bool {
	if t == nil || t.Apply == nil || t.Apply.Overrides == nil {
		return false
	}
	o := t.Apply.Overrides
	return o.TargetConflict != nil || o.ResourceConflict != nil || o.KeepPreviousSurface != nil
}

// HasOverride reports whether the CRDEntry has any override fields set
// at the CRD level or on any target.
func (c *CRDEntry) HasOverride() bool {
	if !c.ServeEnabled() {
		return false
	}

	// Check CRD-level override
	if c.Serve.HasOverride() {
		return true
	}

	// Check any target-level override
	if c.Serve.Target.Entries != nil {
		for _, cfg := range c.Serve.Target.Entries {
			if cfg.HasOverride() {
				return true
			}
		}
	}

	return false
}

// HasTargetOverride reports whether the CRDEntry has any targetConflict set
// at the CRD level or on any target.
func (c *CRDEntry) HasTargetOverride() bool {
	if !c.ServeEnabled() {
		return false
	}

	if c.Serve.Apply != nil && c.Serve.Apply.Overrides != nil && c.Serve.Apply.Overrides.TargetConflict != nil {
		return true
	}

	if c.Serve.Target.Entries != nil {
		for _, cfg := range c.Serve.Target.Entries {
			if cfg.Apply != nil && cfg.Apply.Overrides != nil && cfg.Apply.Overrides.TargetConflict != nil {
				return true
			}
		}
	}

	return false
}

// HasResourceConflict reports whether the CRDEntry has any resourceConflict set
// at the CRD level or on any target.
func (c *CRDEntry) HasResourceConflict() bool {
	if !c.ServeEnabled() {
		return false
	}

	if c.Serve.Apply != nil && c.Serve.Apply.Overrides != nil && c.Serve.Apply.Overrides.ResourceConflict != nil {
		return true
	}

	if c.Serve.Target.Entries != nil {
		for _, cfg := range c.Serve.Target.Entries {
			if cfg.Apply != nil && cfg.Apply.Overrides != nil && cfg.Apply.Overrides.ResourceConflict != nil {
				return true
			}
		}
	}

	return false
}

// KeepPreviousSurface reports whether the CRDEntry or the given target
// has keepPreviousSurface enabled. CRD-level wins if set; otherwise the
// target entry is checked.
func (c *CRDEntry) KeepPreviousSurface(target string) bool {
	if !c.ServeEnabled() {
		return false
	}
	if c.Serve.Apply != nil && c.Serve.Apply.Overrides != nil && c.Serve.Apply.Overrides.KeepPreviousSurface != nil {
		return *c.Serve.Apply.Overrides.KeepPreviousSurface
	}
	if c.Serve.Target.Entries != nil {
		if cfg, ok := c.Serve.Target.Entries[target]; ok {
			if cfg.Apply != nil && cfg.Apply.Overrides != nil && cfg.Apply.Overrides.KeepPreviousSurface != nil {
				return *cfg.Apply.Overrides.KeepPreviousSurface
			}
		}
	}
	return false
}

// EffectiveServeTargetForCR returns the effective target for a given CR.
// Resolution order:
//  1. If the CR matches a target via fieldSelector, use that target.
//  2. Otherwise, fall back to the primary target (serve.target).
//  3. Returns empty string if no target is found.
//
// This is the single source of truth for target resolution across the gateway.
func (c *CRDEntry) EffectiveServeTargetForCR(obj *unstructured.Unstructured) string {
	if !c.ServeEnabled() {
		return ""
	}

	// 1. Try fieldSelector
	if target := c.ServeTargetForFieldSelector(obj.Object); target != "" {
		return target
	}

	// 2. Fall back to primary target
	return c.ServeTarget()
}

// EffectiveServeTargetForMap is the same as EffectiveServeTargetForCR but accepts
// a map[string]interface{} instead of an Unstructured.
func (c *CRDEntry) EffectiveServeTargetForMap(obj map[string]interface{}) string {
	if !c.ServeEnabled() {
		return ""
	}

	if target := c.ServeTargetForFieldSelector(obj); target != "" {
		return target
	}

	return c.ServeTarget()
}

// ─── FieldSelector methods ─────────────────────────────────────────────────────

// HasServeTargetFieldSelector reports whether the ServeTargetConfig has any fieldSelector set.
func (t *ServeTargetConfig) HasServeTargetFieldSelector() bool {
	if t == nil {
		return false
	}
	return len(t.FieldSelector) > 0
}

// Len returns the number of field selectors.
func (t *ServeTargetConfig) Len() int {
	if t == nil {
		return 0
	}
	return len(t.FieldSelector)
}

// HasServeTargetFieldSelector reports whether the CRDEntry has any fieldSelector set on any target.
func (c *CRDEntry) HasServeTargetFieldSelector() bool {
	if !c.ServeEnabled() {
		return false
	}
	if c.Serve.Target.Entries == nil {
		return false
	}
	for _, cfg := range c.Serve.Target.Entries {
		if cfg.HasServeTargetFieldSelector() {
			return true
		}
	}
	return false
}

// FieldSelectorForTarget returns the fieldSelector for a specific target.
func (c *CRDEntry) FieldSelectorForTarget(target string) map[string]string {
	if !c.ServeEnabled() || c.Serve.Target.Entries == nil {
		return nil
	}
	if cfg, ok := c.Serve.Target.Entries[target]; ok {
		return cfg.FieldSelector
	}
	return nil
}

// ServeTargetForFieldSelector returns the target name that matches the given fields.
// Returns empty string if no target matches.
func (c *CRDEntry) ServeTargetForFieldSelector(obj map[string]interface{}) string {
	if !c.ServeEnabled() || c.Serve.Target.Entries == nil {
		return ""
	}

	for targetName, cfg := range c.Serve.Target.Entries {
		if !cfg.HasServeTargetFieldSelector() {
			continue
		}
		if kube.MatchesAllServeTargetFieldSelectors(obj, cfg.FieldSelector) {
			return targetName
		}
	}
	return ""
}
