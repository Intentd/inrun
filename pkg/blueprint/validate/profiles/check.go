// Package profiles checks profiles: resource, probe and user profiles, and the HPA, PDB, rolling update, NetworkPolicy, ResourceQuota and LimitRange profiles.
package profiles

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
		c.validateResourceProfile,
		c.validateProbeProfiles,
		c.validateUserProfiles,
		c.validateHPABehaviorProfiles,
		c.validatePDBBehaviorProfiles,
		c.validateRollingUpdateProfiles,
		c.validateNetworkPolicyProfiles,
		c.validateResourceQuotaProfiles,
		c.validateLimitRangeProfiles,
	)
}
