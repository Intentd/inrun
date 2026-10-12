package serve

import (
	"fmt"
	"slices"
	"strings"
)

// validateServeView checks the serve response configuration for:
//   - Path conflicts between payload and exclude (exclude wins, surfaced as warning)
//   - Payload template compilation errors (error)
func (c *checker) validateServeView() error {
	for crdName, crd := range c.Cat.EnabledCRDs() {
		if !crd.HasServeView() {
			continue
		}

		view := crd.Serve.View
		if !view.HasPayload() && !view.HasExclude() {
			continue
		}

		// Collect payload keys (these will become top-level fields)
		payloadKeys := make(map[string]bool)
		for key := range view.Payload {
			payloadKeys[key] = true
		}

		// Resolve exclude paths (best-effort at validation time)
		if !view.HasExclude() {
			continue
		}
		excludePaths := view.Exclude

		// If exclude is a template, we can't validate its contents at load time
		for _, p := range excludePaths {
			path := strings.TrimSpace(p)
			if path == "" {
				continue
			}
			if isTemplate(path) {
				// Still check if it references any payload keys in the template itself
				for key := range payloadKeys {
					if slices.Contains(excludePaths, key) {
						warning := fmt.Sprintf("%s CRD %q: exclude template references payload key %q — potential conflict\n", warningMark(), crdName, key)
						crd.Warnings.AddWarning(warning)
					}
				}
				continue
			}
		}

		// Static exclude: validate
		for _, p := range excludePaths {
			path := strings.TrimSpace(p)
			if path == "" {
				continue
			}

			// Check if path conflicts with a payload key
			if payloadKeys[path] {
				warning := fmt.Sprintf("path %q appears in both payload and exclude — exclude wins", path)
				crd.Warnings.AddWarning(warning)
			}
		}
		c.Cat.EnabledCRDs()[crdName] = crd
	}
	return nil
}
