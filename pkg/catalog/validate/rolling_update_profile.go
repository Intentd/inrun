// Rolling Update Profile Validation
//
// Rolling update profiles are named presets that expand into MaxSurge and
// MaxUnavailable values on a Deployment's rolling update strategy at Catalog load time.
//
// Validation enforces:
//
// 1. Known profile names:
//    Allowed: safe, fast, blue-green.
//
// 2. Profile-only usage:
//    rollingUpdate.profile cannot appear alongside explicit maxSurge or maxUnavailable.
//
// 3. Template expressions:
//    Profile values containing "{{" are skipped at load time.

package validate

import (
	"fmt"
	"github.com/intentd/inrun/pkg/types"

	"github.com/intentd/inrun/pkg/profiles"
)

func (e *executor) validateRollingUpdateProfiles() error {
	for crdName, crd := range e.cat.EnabledCRDs() {
		for _, entry := range crd.CollectRollingUpdateProfileEntries() {
			if types.IsTemplate(entry.Profile) {
				continue
			}
			if !e.isUserRollingUpdateProfile(entry.Profile) && !profiles.IsValidRollingUpdateProfile(entry.Profile) {
				return fmt.Errorf(
					"%s crd %q: Deployment %q (phase %s) has unknown rollingUpdate.profile %q — "+
						"allowed: safe, fast, blue-green",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: Deployment %q (phase %s) declares both rollingUpdate.profile (%q) and "+
						"explicit maxSurge/maxUnavailable — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
		}
	}
	return nil
}
