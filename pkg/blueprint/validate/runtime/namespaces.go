package runtime

import (
	"fmt"
)

// validateNamespaceProtection enforces that each CRD declares either
// allowedNamespaces or restrictedNamespaces, but not both.
func (c *checker) validateNamespaceProtection() error {
	for name, crd := range c.Blueprint.EnabledCRDs() {
		if !crd.IsNamespaced() || !crd.HasNamespaceRules() {
			continue
		}
		if crd.AllowedNamespacesOnly() || crd.RestrictedNamespacesOnly() {
			continue
		}
		if crd.HasAllowedNamespaces() && crd.HasRestrictedNamespaces() {
			return fmt.Errorf(
				"%s CRD %q cannot define both allowedNamespaces and restrictedNamespaces — choose one",
				failureMark(), name,
			)
		}
	}
	return nil
}
