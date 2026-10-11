package reconcile

import (
	"fmt"

	"github.com/intentd/inrun/pkg/types"
)

// validateService runs all service-related validations.
func (c *checker) validateService() error {
	if err := validateServiceType(c); err != nil {
		return err
	}
	if err := validateServiceProtocol(c); err != nil {
		return err
	}
	return nil
}

//
// ─────────────────────────────────────────────────────────────
// Validation logic
// ─────────────────────────────────────────────────────────────
//

// validateServiceType validates the Service.Type field for all CRDs.
func validateServiceType(c *checker) error {
	for _, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasAnyServices() {
			continue
		}

		// onCreate
		if crd.HasOnCreate() {
			for _, s := range crd.Box().EffectiveOnCreate().Services {
				if !types.IsValidServiceType(s.Type) {
					return errInvalidServiceType(s.Type)
				}
			}
		}

		// onReconcile
		if crd.HasOnReconcile() {
			for _, s := range crd.Box().EffectiveOnReconcile().Services {
				if !types.IsValidServiceType(s.Type) {
					return errInvalidServiceType(s.Type)
				}
			}
		}
	}

	return nil
}

// validateServiceProtocol validates the Service.Protocol field for all CRDs.
func validateServiceProtocol(c *checker) error {
	for _, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasAnyServices() {
			continue
		}

		// onCreate
		if crd.HasOnCreate() {
			for _, s := range crd.Box().EffectiveOnCreate().Services {
				if !types.IsValidProtocol(s.Protocol) {
					return errInvalidProtocol(s.Protocol)
				}
			}
		}

		// onReconcile
		if crd.HasOnReconcile() {
			for _, s := range crd.Box().EffectiveOnReconcile().Services {
				if !types.IsValidProtocol(s.Protocol) {
					return errInvalidProtocol(s.Protocol)
				}
			}
		}
	}

	return nil
}

//
// ─────────────────────────────────────────────────────────────
// Reusable error helpers
// ─────────────────────────────────────────────────────────────
//

func errInvalidServiceType(t string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s Invalid Service type: %q

Allowed values:
  • ClusterIP
  • NodePort
  • LoadBalancer

Docs: https://kubernetes.io/docs/concepts/services-networking/service/
──────────────────────────────────────────────`, failureMark(), t)
}

func errInvalidProtocol(p string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s Invalid protocol: %q

Allowed values:
  • TCP
  • UDP
  • SCTP

Docs: https://kubernetes.io/docs/concepts/services-networking/service/#protocol
──────────────────────────────────────────────`, failureMark(), p)
}
