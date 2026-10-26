package types

import "strings"

// ServeTarget returns the identifier callers use to reference this CRD in the
// Gateway API and schema API.
//
// Resolution order:
//  1. The primary entry's map key in serve.target (map form).
//  2. serve.target shorthand string (before load-time expansion).
//  3. Lowercased kind — "App" → "app", "DatabaseCluster" → "databasecluster".
//
// Returns an empty string when target mode is disabled (serve.modes.target: false).
// This ensures that when target mode is turned off, the CRD has no target
// regardless of what the configuration declares.
func (c *CRDEntry) ServeTarget() string {
	if c.Serve == nil {
		return strings.ToLower(c.APITypes.Kind)
	}
	if !c.TargetModeEnabled() {
		return ""
	}
	// Shorthand (before scalar expansion at load time).
	if c.Serve.Target.Shorthand != "" {
		return c.Serve.Target.Shorthand
	}
	// Map form — find the entry marked primary: true.
	for name, cfg := range c.Serve.Target.Entries {
		if cfg != nil && cfg.Primary {
			return name
		}
	}
	return strings.ToLower(c.APITypes.Kind)
}

// HasServeTarget reports whether this CRD can be addressed by its primary target.
// Returns false when:
//   - serve is disabled
//   - kind is absent
//   - target mode is disabled (serve.modes.target: false)
//   - the primary entry's enabled flag is false
//
// A disabled primary means the CRD is only reachable via its alias entries.
func (c *CRDEntry) HasServeTarget() bool {
	if !c.ServeEnabled() || c.APITypes.Kind == "" || !c.TargetModeEnabled() {
		return false
	}

	if c.Serve.Target.IsZero() {
		return true // no target config at all → default enabled
	}
	if c.Serve.Target.Shorthand != "" {
		return true // scalar shorthand → always enabled
	}
	pt := c.PrimaryTarget()
	return pt == nil || pt.IsEnabled()
}

// ServeTargetOrEmpty returns ServeTarget when HasServeTarget is true, "" otherwise.
func (c *CRDEntry) ServeTargetOrEmpty() string {
	if !c.HasServeTarget() {
		return ""
	}
	return c.ServeTarget()
}

// FirstServeTargetEntry returns the first serve entry in the map
func (c *CRDEntry) FirstServeTargetEntry() *ServeTargetConfig {
	if c.Serve == nil || !c.TargetModeEnabled() {
		return nil
	}
	for _, cfg := range c.Serve.Target.Entries {
		return cfg
	}
	return nil
}

// PrimaryTarget returns the TargetConfig whose Primary flag is true, or nil when
// no primary entry is declared (scalar shorthand, or no target configured).
func (c *CRDEntry) PrimaryTarget() *ServeTargetConfig {
	if c.Serve == nil || !c.TargetModeEnabled() {
		return nil
	}
	for _, cfg := range c.Serve.Target.Entries {
		if cfg != nil && cfg.Primary {
			return cfg
		}
	}
	return nil
}

// AllServeTargets returns the full Target.Entries map, including disabled entries.
// Intended for CLI display only — callers that resolve requests should use
// LookupTarget, which filters disabled entries.
func (c *CRDEntry) AllServeTargets() map[string]*ServeTargetConfig {
	if c.Serve == nil || !c.TargetModeEnabled() {
		return nil
	}
	return c.Serve.Target.Entries
}

// LookupTarget returns the TargetConfig for the given entry name if it is enabled.
// Returns nil for disabled entries, the primary when name matches, and unknown names.
func (c *CRDEntry) LookupTarget(name string) *ServeTargetConfig {
	if c.Serve == nil || !c.TargetModeEnabled() {
		return nil
	}
	cfg, ok := c.Serve.Target.Entries[name]
	if !ok || !cfg.IsEnabled() {
		return nil
	}
	return cfg
}

// AllServeFields returns a flat map of every serve-declared field — spec fields from
// serve.fields, label fields from serve.labels, and annotation
// fields from serve.annotations.
//
// Used by the schema API to expose the complete field contract to callers and
// by BuildCRFromTarget to route each submitted value to the right CR path.
//
// Fields from the same key in multiple sources are merged in this order:
// spec fields → label fields → annotation fields. In practice, the platform
// team should not declare the same key in more than one source; inrun validate
// catches this.
func (c *CRDEntry) AllServeFields() map[string]ServeFieldConfig {
	if c.Serve == nil {
		return nil
	}
	out := make(map[string]ServeFieldConfig, len(c.Serve.Fields))

	// Spec fields — declared under serve.fields.
	for name, cfg := range c.Serve.Fields {
		out[name] = cfg
	}

	// Label fields — declared under serve.labels.
	for name, cfg := range c.ServeLabels() {
		out[name] = cfg
	}

	// Annotation fields — declared under serve.annotations.
	for name, cfg := range c.ServeAnnotations() {
		out[name] = cfg
	}

	return out
}
