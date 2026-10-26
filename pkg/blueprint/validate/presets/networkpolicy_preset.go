// NetworkPolicy Preset Validation
//
// NetworkPolicy presets are named presets that expand into a complete set of
// ingress/egress rules and policy types at reconcile time.
//
// Validation enforces:
//
// 1. Known preset names:
//    Allowed: deny-all, deny-all-ingress, deny-all-egress,
//             allow-same-namespace, allow-dns-egress.
//
// 2. Preset-only usage:
//    preset cannot appear alongside explicit ingress, egress, or policyTypes
//    fields — presets are atomic presets.
//
// 3. Template expressions:
//    A preset value that is a template must parse; the name it produces is
//    checked when rendered.

package presets

import (
	"fmt"
)

func (c *checker) validateNetworkPolicyPresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectNetworkPolicyPresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("networkPolicy.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			if !c.isUserNetworkPolicyPreset(entry.Preset) && !isBuiltinNetworkPolicy(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: networkPolicy %q (phase %s) has unknown preset %q — "+
						"allowed: deny-all, deny-all-ingress, deny-all-egress, allow-same-namespace, allow-dns-egress",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: networkPolicy %q (phase %s) declares both preset (%q) and "+
						"explicit ingress/egress/policyTypes — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
