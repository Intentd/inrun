package validate

import (
	"github.com/inrundev/inrun/pkg/external"
)

func (e *executor) validateExternalCalls() error {
	for crdName, entry := range e.k.EnabledCRDs() {
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
