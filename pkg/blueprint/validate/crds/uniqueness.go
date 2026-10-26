package crds

import (
	"fmt"
)

func (c *checker) validateUniqueness() error {
	// Name uniqueness is guaranteed by map keys — only check GVK uniqueness.
	return c.validateGVKUniqueness()
}

func (c *checker) validateGVKUniqueness() error {
	seen := make(map[string]string) // key -> name

	for name, crd := range c.Blueprint.EnabledCRDs() {
		key := fmt.Sprintf("%s/%s/%s", crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Kind)

		if existing, ok := seen[key]; ok {
			return fmt.Errorf(
				"duplicate GVK detected: %s/%s, Kind=%s (CRDs: %s and %s)",
				crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Kind, existing, name,
			)
		}

		seen[key] = name
	}

	return nil
}
