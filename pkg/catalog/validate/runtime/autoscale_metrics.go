package runtime

import (
	"fmt"
	"strings"
)

// validateAutoscalerMetrics ensures only supported metrics.* fields are used.
// This is a fail-fast mechanism to avoid runtime errors.
func (c *checker) validateAutoscalerMetrics() error {
	for _, crd := range c.Cat.EnabledCRDs() {
		if !crd.AutoscaleEnabled() {
			continue
		}

		conds := crd.Box().EffectiveAutoscale().Conditions

		for _, cond := range conds.Or {
			if strings.HasPrefix(cond.Field, "metrics.") {
				if err := crd.ValidateMetricField(cond.Field); err != nil {
					handleValidationErrors(err)
					return fmt.Errorf(failureMark(), err)
				}
			}
		}

		for _, cond := range conds.When {
			if strings.HasPrefix(cond.Field, "metrics.") {
				if err := crd.ValidateMetricField(cond.Field); err != nil {
					handleValidationErrors(err)
					return fmt.Errorf(failureMark(), err)
				}
			}
		}
	}
	return nil
}
