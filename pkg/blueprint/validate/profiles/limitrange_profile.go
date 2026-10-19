// LimitRange Profile Validation
//
// LimitRange profiles are user-defined named presets that expand into a
// list of LimitRangeItems at reconcile time. There are no built-in presets —
// every limitRange profile must be declared in the blueprint or an imported module.
//
// Validation enforces:
//
// 1. Known profile names:
//    Profile must appear in profiles.limitRanges or an imported module.
//
// 2. Profile-only usage:
//    profile cannot appear alongside an explicit limits list.
//
// 3. Template expressions:
//    Profile values containing "{{" are skipped at load time.

package profiles

import (
	"fmt"
)

func (c *checker) validateLimitRangeProfiles() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		for _, entry := range crd.CollectLimitRangeProfileEntries() {
			if isTemplate(entry.Profile) {
				continue
			}
			if !c.isUserLimitRangeProfile(entry.Profile) {
				return fmt.Errorf(
					"%s crd %q: LimitRange %q (phase %s) has unknown profile %q — "+
						"define it in profiles.limitRanges",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: LimitRange %q (phase %s) declares both profile (%q) and "+
						"explicit limits — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
		}
	}
	return nil
}
