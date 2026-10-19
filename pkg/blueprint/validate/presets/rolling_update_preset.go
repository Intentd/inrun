// Rolling Update Preset Validation
//
// Rolling update presets are named presets that expand into MaxSurge and
// MaxUnavailable values on a Deployment's rolling update strategy at Blueprint load time.
//
// Validation enforces:
//
// 1. Known preset names:
//    Allowed: safe, fast, blue-green.
//
// 2. Preset-only usage:
//    rollingUpdate.preset cannot appear alongside explicit maxSurge or maxUnavailable.
//
// 3. Template expressions:
//    Preset values containing "{{" are skipped at load time.

package presets

import (
	"fmt"
)

func (c *checker) validateRollingUpdatePresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectRollingUpdatePresetEntries() {
			if isTemplate(entry.Preset) {
				continue
			}
			if !c.isUserRollingUpdatePreset(entry.Preset) && !isBuiltinRollingUpdate(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: Deployment %q (phase %s) has unknown rollingUpdate.preset %q — "+
						"allowed: safe, fast, blue-green",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: Deployment %q (phase %s) declares both rollingUpdate.preset (%q) and "+
						"explicit maxSurge/maxUnavailable — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
