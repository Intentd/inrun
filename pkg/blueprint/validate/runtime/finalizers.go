package runtime

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// validateFinalizerTimeout checks runtime.finalizers.timeout on each CRD and
// serve target: a duration accepted by core.ParseTimeDuration (KEL), greater than
// zero. Empty is allowed — no timeout.
func (c *checker) validateFinalizerTimeout() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		check := func(where string, box *types.OperatorBoxConfig) error {
			f := box.EffectiveFinalizerConfig()
			if f == nil || f.Timeout == "" {
				return nil
			}
			if d, err := parseTimeDuration(f.Timeout); err != nil || d <= 0 {
				return fmt.Errorf("%s crd %q: %sruntime.finalizers.timeout %q is not a positive duration. "+
					"Use a duration string (e.g. \"30m\", \"2h\") or leave it unset for no timeout",
					failureMark(), crdName, where, f.Timeout)
			}
			return nil
		}
		if err := check("", crd.OperatorBox); err != nil {
			return err
		}
		if crd.Serve != nil {
			for target, entry := range crd.Serve.Target.Entries {
				if entry == nil {
					continue
				}
				if err := check(fmt.Sprintf("serve.target[%s].", target), entry.OperatorBox); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
