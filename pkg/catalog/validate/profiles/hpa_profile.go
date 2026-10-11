// HPA Behavior Profile Validation
//
// HPA behavior profiles are named presets that expand into a complete
// HorizontalPodAutoscalerBehavior block (scaleUp, scaleDown policies and
// stabilization windows) plus a suggested CPU utilization target.
//
// Validation enforces:
//
// 1. Known profile names:
//    Allowed: web, api, latency-sensitive, batch, cost-optimized.
//
// 2. Profile-only usage:
//    behavior.profile cannot appear alongside explicit scaleUp or scaleDown
//    fields — profiles are atomic presets.
//
// 3. Template expressions:
//    Profile values containing "{{" are skipped at load time and validated
//    at reconcile time instead.

package profiles

import (
	"fmt"

	"github.com/intentd/inrun/pkg/profiles"
)

func (c *checker) validateHPABehaviorProfiles() error {
	for crdName, crd := range c.Cat.EnabledCRDs() {
		for _, entry := range crd.CollectHPAProfileEntries() {
			if isTemplate(entry.Profile) {
				continue
			}
			if !c.isUserHPAProfile(entry.Profile) && !profiles.IsValidHPAProfile(entry.Profile) {
				return fmt.Errorf(
					"%s crd %q: HPA %q (phase %s) has unknown behavior.profile %q — "+
						"allowed: web, api, latency-sensitive, batch, cost-optimized",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
			if entry.Mixed {
				return fmt.Errorf(
					"%s crd %q: HPA %q (phase %s) declares both behavior.profile (%q) and "+
						"explicit scaleUp/scaleDown — use one or the other, not both",
					failureMark(), crdName, entry.ResourceName, entry.Phase, entry.Profile,
				)
			}
		}
	}
	return nil
}
