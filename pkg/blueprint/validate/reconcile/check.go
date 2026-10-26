// Package reconcile checks operatorBox.reconcile: the reconciler and its registrations, external calls, retries, requeue, enrich, and the resource templates.
package reconcile

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
		c.validateReconciler,
		c.validateRegistrations,
		c.validateExternalCalls,
		c.validateRetryBackoff,
		c.validateRequeue,
		c.validateReconcileConditions,
		c.validateEnrich,
		c.validateTimeDuration,
		c.validateHPAReference,
		c.validateService,
		c.validateCustomResources,
		c.validateSecurityPresets,
		c.validateSecurityCapabilities,
		c.validatePortProtocols,
		c.validateWorkloadAutoscale,
		c.validateEnvFromRefs,
	)
}
