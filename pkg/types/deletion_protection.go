package types

// DeletionProtectionOverride controls per‑CRD behaviour when the global
// security.deletionProtection.enabled is true.
// Both fields default to true when omitted.
type DeletionProtectionOverride struct {
	// ProtectCRD determines whether the CRD definition itself (the type)
	// is protected from deletion. Default true.
	ProtectCRD *bool `yaml:"protectCRD,omitempty" json:"protectCRD,omitempty"`

	// ProtectCRs determines whether instances of this CRD are protected
	// from deletion (via the inrun.dev/deletion-protection label).
	// Default true.
	ProtectCRs *bool `yaml:"protectCRs,omitempty" json:"protectCRs,omitempty"`

	// StrictMode controls whether removing the deletion-protection label from a resource
	// is itself treated as a deletion attempt and blocked.
	// When true, the only way to remove the label (and thus unprotect a resource) is to
	// disable strictMode in the Blueprint and restart Inrun Gateway.
	// Default: blueprint level strictMode.
	StrictMode *bool `yaml:"strictMode,omitempty" json:"strictMode,omitempty"`
}

// deletionProtection returns the effective DeletionProtectionOverride from operatorBox.runtime.
func (c *CRDEntry) deletionProtection() *DeletionProtectionOverride {
	if c.OperatorBox.Runtime == nil {
		return nil
	}
	return c.OperatorBox.Runtime.DeletionProtection
}

// HasDeletionProtectionOverride reports whether deletion protection override is set for this CRD.
func (c *CRDEntry) HasDeletionProtectionOverride() bool {
	return c.deletionProtection() != nil
}

// ShouldProtectCRD reports whether the CRD *type definition* itself should be protected.
// Defaults to true when not configured.
func (c *CRDEntry) ShouldProtectCRD() bool {
	dp := c.deletionProtection()
	if dp == nil || dp.ProtectCRD == nil {
		return true
	}
	return *dp.ProtectCRD
}

// ShouldProtectCRs reports whether *instances* of this CRD should be protected.
// Defaults to true when not configured.
func (c *CRDEntry) ShouldProtectCRs() bool {
	dp := c.deletionProtection()
	if dp == nil || dp.ProtectCRs == nil {
		return true
	}
	return *dp.ProtectCRs
}

// IsStrictDeletionProtection returns whether strict deletion-protection semantics apply.
func (c *CRDEntry) IsStrictDeletionProtection(blueprintStrictMode bool) bool {
	if !blueprintStrictMode {
		return false
	}
	dp := c.deletionProtection()
	if dp == nil || dp.StrictMode == nil {
		return true
	}
	return *dp.StrictMode
}
