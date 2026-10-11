package reconcile

import (
	"github.com/intentd/inrun/pkg/external"
)

func (c *checker) validateExternalCalls() error {
	for crdName, entry := range c.Cat.EnabledCRDs() {
		if ht := entry.OperatorBox.EffectiveOnReconcile(); ht != nil {
			if err := external.ValidateCalls(crdName, "onReconcile.external", ht.External); err != nil {
				return err
			}
		}
		if ht := entry.OperatorBox.EffectiveOnCreate(); ht != nil {
			if err := external.ValidateCalls(crdName, "onCreate.external", ht.External); err != nil {
				return err
			}
		}
		if r := entry.OperatorBox.Reconcile; r != nil {
			if err := external.ValidateCalls(crdName, "reconcile.external", r.External); err != nil {
				return err
			}
		}
		if v := entry.EffectiveValidation(); v != nil {
			if err := external.ValidateCalls(crdName, "validation.external", v.External); err != nil {
				return err
			}
		}
		if m := entry.EffectiveMutation(); m != nil {
			if err := external.ValidateCalls(crdName, "mutation.external", m.External); err != nil {
				return err
			}
		}
	}
	return nil
}
