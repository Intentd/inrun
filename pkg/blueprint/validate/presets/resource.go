// Resource Preset Validation
//
// Resource presets are computed presets. When a preset is declared under
// resources.preset, it must be the *only* resource input. Presets expand
// into a complete ResourceRequirements struct during blueprint enrichment, so
// mixing them with manual fields creates ambiguity.
//
// Validation uses the same visitor pattern as probe preset validation:
// CollectResourcePresetEntries iterates every hook phase and resource,
// surfacing entries where resources.preset is non-empty.
//
// Validation enforces:
//
// 1. Preset-only usage:
//    resources.preset cannot appear alongside requests or limits.
//
// 2. Known preset names:
//    Allowed: tiny, small, medium, large, burst, steady, compute-heavy,
//    memory-heavy.
//
// 3. Template expressions:
//    A preset value that is a template must parse; the name it produces is
//    checked when rendered.

package presets

import (
	"fmt"
)

func (c *checker) validateResourcePreset() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectResourcePresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("resources.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			_, userDefined := c.Blueprint.Presets.Resource(entry.Preset)
			if !userDefined && !isBuiltinResource(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: %s %q (phase %s) has unknown resources.preset %q — "+
						"allowed: tiny, small, medium, large, burst, steady, compute-heavy, memory-heavy, or a user-defined preset declared in presets.resources",
					failureMark(), crdName, entry.Resource, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: %s %q (phase %s) declares both resources.preset (%q) and "+
						"explicit requests/limits — use one or the other, not both",
					failureMark(), crdName, entry.Resource, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}

func (c *checker) validateProbePresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectProbePresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("probe.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			_, userDefined := c.Blueprint.Presets.Probe(entry.Preset)
			if !userDefined && !isBuiltinProbe(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: %s probe in %s %q (phase %s) has unknown probe preset %q — "+
						"allowed: fast, standard, patient, slow-start, or a user-defined preset declared in presets.probes",
					failureMark(), crdName, entry.ProbeType, entry.Resource, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: %s probe in %s %q (phase %s) declares both a preset (%q) and "+
						"explicit timing fields — use one or the other, not both",
					failureMark(), crdName, entry.ProbeType, entry.Resource, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
