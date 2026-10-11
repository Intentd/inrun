// Package runtime checks operatorBox.runtime: autoscale, allowed and restricted namespaces, cross-namespace writes, and finalizers.
package runtime

import (
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate/internal/check"
	"github.com/intentd/inrun/pkg/config"
)

type checker struct{ check.Checker }

// Check runs this block's checks in order and returns the first error.
func Check(cat *catalog.Catalog, cfg *config.Config) error {
	c := &checker{check.New(cat, cfg)}
	return check.First(
		c.validateAutoscaleProfile,
		c.validateAutoscalerMetrics,
		c.validateNamespaceProtection,
		c.validateCrossNamespaceOps,
		c.validateFinalizerTimeout,
	)
}
