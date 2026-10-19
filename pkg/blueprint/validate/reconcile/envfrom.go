package reconcile

import (
	"fmt"
)

// validateEnvFromRefs checks that no envFrom secretRef/configMapRef entry
// declares suffix without keys — Kubernetes cannot rename keys during a
// blanket envFrom import, so suffix only makes sense alongside an explicit
// keys list (which expands into individual env entries instead of a native
// envFrom source).
func (c *checker) validateEnvFromRefs() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectEnvFromEntries() {
			if entry.Ref.Suffix != "" && len(entry.Ref.Keys) == 0 {
				return errSuffixRequiresKeys(crdName, entry.ResourceName, entry.Phase, entry.RefKind, entry.Ref.Name)
			}
		}
	}
	return nil
}

func errSuffixRequiresKeys(crd, resource, phase, refKind, refName string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s envFrom.%s %q declares suffix without keys
   CRD:      %s
   Resource: %s
   Phase:    %s

Kubernetes cannot rename keys during a blanket envFrom import — suffix only
applies when keys also selects which keys to expand individually.

Fix: add a keys: list alongside suffix, or remove suffix for a blanket import.
──────────────────────────────────────────────`, failureMark(), refKind, refName, crd, resource, phase)
}
