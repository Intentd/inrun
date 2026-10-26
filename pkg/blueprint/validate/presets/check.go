// Package presets checks a Blueprint's presets: that each name it uses exists, built in or declared under presets:, and that its own presets do not clash.
package presets

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate/internal/check"
	"github.com/intentd/inrun/pkg/config"
)

type checker struct{ check.Checker }

// Check runs this block's checks in order and returns the first error.
func Check(bp *blueprint.Blueprint, cfg *config.Config) error {
	c := &checker{check.New(bp, cfg)}
	return check.First(
		c.validateResourcePreset,
		c.validateProbePresets,
		c.validateUserPresets,
		c.validateHPABehaviorPresets,
		c.validatePDBBehaviorPresets,
		c.validateRollingUpdatePresets,
		c.validateNetworkPolicyPresets,
		c.validateResourceQuotaPresets,
		c.validateLimitRangePresets,
	)
}
