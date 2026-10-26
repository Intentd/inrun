// HPA Behavior Preset Validation
//
// HPA behavior presets are named presets that expand into a complete
// HorizontalPodAutoscalerBehavior block (scaleUp, scaleDown policies and
// stabilization windows) plus a suggested CPU utilization target.
//
// Validation enforces:
//
// 1. Known preset names:
//    Allowed: web, api, latency-sensitive, batch, cost-optimized.
//
// 2. Preset-only usage:
//    behavior.preset cannot appear alongside explicit scaleUp or scaleDown
//    fields — presets are atomic presets.
//
// 3. Template expressions:
//    A preset value that is a template must parse; the name it produces is
//    checked when rendered.

package presets

import (
	"fmt"
)

func (c *checker) validateHPABehaviorPresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectHPAPresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("hpa.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			if !c.isUserHPAPreset(entry.Preset) && !isBuiltinHPA(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: HPA %q (phase %s) has unknown behavior.preset %q — "+
						"allowed: web, api, latency-sensitive, batch, cost-optimized",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: HPA %q (phase %s) declares both behavior.preset (%q) and "+
						"explicit scaleUp/scaleDown — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
