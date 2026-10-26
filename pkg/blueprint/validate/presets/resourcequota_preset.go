// ResourceQuota Preset Validation
//
// ResourceQuota presets are named presets that expand into a complete set of
// hard resource limits at reconcile time.
//
// Validation enforces:
//
// 1. Known preset names:
//    Allowed: small, medium, large, xlarge.
//
// 2. Preset-only usage:
//    preset cannot appear alongside an explicit hard map — presets are
//    atomic presets.
//
// 3. Template expressions:
//    A preset value that is a template must parse; the name it produces is
//    checked when rendered.

package presets

import (
	"fmt"
)

func (c *checker) validateResourceQuotaPresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectResourceQuotaPresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("resourceQuota.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			if !c.isUserResourceQuotaPreset(entry.Preset) && !isBuiltinResourceQuota(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: resourceQuota %q (phase %s) has unknown preset %q — "+
						"allowed: small, medium, large, xlarge",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: resourceQuota %q (phase %s) declares both preset (%q) and "+
						"explicit hard limits — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
