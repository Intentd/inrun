// LimitRange Preset Validation
//
// LimitRange presets are user-defined named presets that expand into a
// list of LimitRangeItems at reconcile time. There are no built-in presets —
// every limitRange preset must be declared in the blueprint or an imported module.
//
// Validation enforces:
//
// 1. Known preset names:
//    Preset must appear in presets.limitRanges or an imported module.
//
// 2. Preset-only usage:
//    preset cannot appear alongside an explicit limits list.
//
// 3. Template expressions:
//    A preset value that is a template must parse; the name it produces is
//    checked when rendered.

package presets

import (
	"fmt"
)

func (c *checker) validateLimitRangePresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectLimitRangePresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("limitRange.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			if !c.isUserLimitRangePreset(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: LimitRange %q (phase %s) has unknown preset %q — "+
						"define it in presets.limitRanges",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: LimitRange %q (phase %s) declares both preset (%q) and "+
						"explicit limits — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
