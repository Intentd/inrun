package reconcile

import (
	"fmt"

	"github.com/intentd/inrun/pkg/blueprint"
)

// validateRegistrations checks that every compiled piece a CRD runs is
// registered: its object types, and outside the gateway its constructors and
// hooks. It only reads; the runtime looks the same entries up through the
// CRDEntry methods when it builds reconcilers and informers. The gateway
// reconciles nothing and ships no compiled code, so it checks object types
// only (every CR is unstructured there).
func (c *checker) validateRegistrations() error {
	gateway := c.Cfg != nil && c.Cfg.IsGatewayInstance()
	for name, crd := range c.Blueprint.EnabledCRDs() {
		if !gateway {
			if crd.NeedsConstructor() {
				if _, ok := crd.Constructor(); !ok {
					return fmt.Errorf(
						"CRD %q: no constructor registered — "+
							"check reconciler.constructor in Blueprint and re-run inrun generate registry",
						name,
					)
				}
			}
			if _, err := crd.TargetConstructors(); err != nil {
				return fmt.Errorf("CRD %q %w", name, err)
			}
		}

		if _, _, err := crd.RuntimeObjects(); err != nil {
			if crd.RegistryRef != "" {
				return &blueprint.TypedOperatorError{Ref: crd.RegistryRef, Err: err}
			}
			return err
		}

		if !gateway {
			if err := crd.CheckSharedTargetHooks(); err != nil {
				return fmt.Errorf("CRD %q %w", name, err)
			}
			if _, err := crd.TargetHookFactories(); err != nil {
				return fmt.Errorf("CRD %q %w", name, err)
			}
		}
	}
	return nil
}
