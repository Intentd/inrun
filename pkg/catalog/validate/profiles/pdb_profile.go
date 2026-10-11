// PDB Behavior Profile Validation
//
// PDB behavior profiles are named presets that expand into a concrete
// minAvailable or maxUnavailable value at Catalog load time.
//
// Validation enforces:
//
// 1. Known profile names:
//    Allowed: zero-downtime, rolling, relaxed.
//
// 2. Profile-only usage:
//    behavior.profile cannot appear alongside explicit minAvailable or maxUnavailable.
//
// 3. Template expressions:
//    Profile values containing "{{" are skipped at load time.

package profiles

import (
	"fmt"

	"github.com/intentd/inrun/pkg/profiles"
)

func (c *checker) validatePDBBehaviorProfiles() error {
	for crdName, crd := range c.Cat.EnabledCRDs() {
		for _, entry := range crd.CollectPDBProfileEntries() {
			if isTemplate(entry.Profile) {
				continue
			}
			if !c.isUserPDBProfile(entry.Profile) && !profiles.IsValidPDBProfile(entry.Profile) {
				return fmt.Errorf(
					"%s crd %q: PDB %q (phase %s) has unknown behavior.profile %q — "+
						"allowed: zero-downtime, rolling, relaxed",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: PDB %q (phase %s) declares both behavior.profile (%q) and "+
						"explicit minAvailable/maxUnavailable — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
		}
	}
	return nil
}
