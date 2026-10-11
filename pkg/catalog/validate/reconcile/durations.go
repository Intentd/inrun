package reconcile

import (
	"fmt"
)

// validateTimeDuration validates all rotation-related duration fields
// across all enabled CRDs.
func (c *checker) validateTimeDuration() error {
	for name, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasAnyHookTemplates() {
			continue
		}

		for _, entry := range crd.CollectSleepEntries() {
			if isTemplate(entry.Duration) {
				continue
			}
			if _, err := parseTimeDuration(entry.Duration); err != nil {
				return durationError(name, entry.ResourceName, "sleep", entry.Duration, err)
			}
		}

		if !crd.HasAnySecrets() {
			continue
		}
		if crd.HasOnCreate() {
			for _, s := range crd.Box().EffectiveOnCreate().Secrets {
				if s.RotateAfter != "" {
					if _, err := parseTimeDuration(s.RotateAfter); err != nil {
						return durationError(name, s.Name, "rotateAfter", s.RotateAfter, err)
					}
				}
				if s.TLS != nil && s.TLS.ValidFor != "" {
					if _, err := parseTimeDuration(s.TLS.ValidFor); err != nil {
						return durationError(name, s.Name, "validFor", s.TLS.ValidFor, err)
					}
				}
			}
		}

		if crd.HasOnReconcile() {
			for _, s := range crd.Box().EffectiveOnReconcile().Secrets {
				if s.RotateAfter != "" {
					if _, err := parseTimeDuration(s.RotateAfter); err != nil {
						return durationError(name, s.Name, "rotateAfter", s.RotateAfter, err)
					}
				}
				if s.TLS != nil && s.TLS.ValidFor != "" {
					if _, err := parseTimeDuration(s.TLS.ValidFor); err != nil {
						return durationError(name, s.Name, "validFor", s.TLS.ValidFor, err)
					}
				}
			}
		}
	}
	return nil
}

func durationError(crdName, secretName, field, value string, err error) error {
	return fmt.Errorf(
		"%s invalid duration %q in CRD %q (secret %q, field %q): %v\n\n"+
			"Allowed units:\n"+
			"  d   = days (24h)\n"+
			"  w   = weeks (7d)\n"+
			"  mo  = months (30d)\n"+
			"  y   = years (365d)\n\n"+
			"Examples: 30d, 2w, 3mo, 1y",
		failureMark(), value, crdName, secretName, field, err,
	)
}
