// PDB Behavior Preset Validation
//
// PDB behavior presets are named presets that expand into a concrete
// minAvailable or maxUnavailable value at Blueprint load time.
//
// Validation enforces:
//
// 1. Known preset names:
//    Allowed: zero-downtime, rolling, relaxed.
//
// 2. Preset-only usage:
//    behavior.preset cannot appear alongside explicit minAvailable or maxUnavailable.
//
// 3. Template expressions:
//    A preset value that is a template must parse; the name it produces is
//    checked when rendered.

package presets

import (
	"fmt"
)

func (c *checker) validatePDBBehaviorPresets() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectPDBPresetEntries() {
			if isTemplate(entry.Preset) {
				if err := validateTemplate("pdb.preset", crdName, entry.ResourceName, "preset", entry.Preset, funcMap(c.Blueprint.Functions)); err != nil {
					return err
				}
				continue
			}
			if !c.isUserPDBPreset(entry.Preset) && !isBuiltinPDB(entry.Preset) {
				return fmt.Errorf(
					"%s crd %q: PDB %q (phase %s) has unknown behavior.preset %q — "+
						"allowed: zero-downtime, rolling, relaxed",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: PDB %q (phase %s) declares both behavior.preset (%q) and "+
						"explicit minAvailable/maxUnavailable — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Preset,
				)
			}
		}
	}
	return nil
}
