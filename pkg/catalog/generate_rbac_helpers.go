package catalog

import (
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// HasConversionPaths returns true only if conversion is enabled and at least
// one CRD declares conversion paths.
func (c *Catalog) HasConversionPaths() bool {
	if c == nil || c.config == nil {
		logger.Debug().Msg("catalog or config is nil")
		return false
	}
	if !c.IsConversionEnabled() {
		logger.Debug().Msg("conversion is disabled")
		return false
	}
	for _, crd := range c.Enabled() {
		if crd.EffectiveConversion() == nil {
			logger.Debug().Str("crd", crd.Name).Msg("conversion is nil")
			continue
		}
		if len(crd.EffectiveConversion().Paths) > 0 {
			logger.Debug().Str("crd", crd.Name).Msg("conversion has paths")
			return true
		}
	}
	return false
}

// HasValidationRules returns true if admission is enabled and at least one CRD
// declares validation rules.
func (c *Catalog) HasValidationRules() bool {
	if c == nil {
		return false
	}
	if !c.IsAdmissionEnabled() {
		logger.Debug().Msg("admission is disabled")
		return false
	}
	for _, crd := range c.Enabled() {
		if crd.EffectiveValidation() == nil {
			continue
		}
		if len(crd.EffectiveValidation().Rules) > 0 {
			logger.Debug().Str("crd", crd.Name).Msg("validation has rules")
			return true
		}
	}
	return false
}

// HasMutationRules returns true if admission is enabled and at least one CRD
// declares mutation rules.
func (c *Catalog) HasMutationRules() bool {
	if c == nil {
		return false
	}
	if !c.IsAdmissionEnabled() {
		logger.Debug().Msg("admission is disabled")
		return false
	}
	for _, crd := range c.Enabled() {
		if crd.EffectiveMutation() == nil {
			continue
		}
		if len(crd.EffectiveMutation().Rules) > 0 {
			logger.Debug().Str("crd", crd.Name).Msg("mutation has rules")
			return true
		}
	}
	return false
}

// HasValidationOrMutationRules returns true if any CRD has validation or mutation rules.
func (c *Catalog) HasValidationOrMutationRules() bool {
	if c == nil {
		return false
	}
	return c.HasValidationRules() || c.HasMutationRules()
}

// Uses reports whether any enabled CRD uses the given resource in a hook
// template block. Accepts both singular ("deployment") and plural ("deployments")
// keys, as well as shorthands ("hpa"). Drives off builtInRegistry — no separate
// table to maintain.
func (c *Catalog) Uses(resource string) bool {
	b, ok := children.LookupBuiltInByResource(resource)
	if !ok {
		return false
	}
	return c.anyDetects(b.Detect)
}

// anyDetects returns true if detect returns true for any enabled CRD.
func (c *Catalog) anyDetects(detect func(types.CRDEntry) bool) bool {
	for _, crd := range c.Enabled() {
		if detect(crd) {
			return true
		}
	}
	return false
}
