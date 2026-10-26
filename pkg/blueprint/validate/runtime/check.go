// Package runtime checks operatorBox.runtime: autoscale, allowed and restricted namespaces, cross-namespace writes, and finalizers.
package runtime

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
		c.validateAutoscalePreset,
		c.validateAutoscalerMetrics,
		c.validateNamespaceProtection,
		c.validateCrossNamespaceOps,
		c.validateFinalizerTimeout,
	)
}
