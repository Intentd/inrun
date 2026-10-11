package serve

import (
	"fmt"
)

// validateServeFieldPaths validates serve.fields path configurations:
//   - Paths (if set) must be unique across all fields
//   - Path must be a valid dot-notation path (no empty segments, no leading/trailing dots)
//   - Warn if path is nested (contains a dot) — schema existence validation is not yet implemented
func (c *checker) validateServeFieldPaths() error {
	for crdName, crd := range c.Cat.EnabledCRDs() {
		if !crd.ServeEnabled() {
			continue
		}

		if !crd.HasServeFields() {
			continue
		}

		seenPaths := make(map[string]bool)

		for name, config := range crd.Serve.Fields {
			// 1. Skip if no path is set (flat field)
			if !config.HasSpecPath() {
				continue
			}

			specPath := config.SpecPath(name)

			// 2. Validate the path format (no empty segments)
			if err := validatePathFormat(specPath); err != nil {
				return fmt.Errorf(
					"%s CRD %q: serve.fields %q has invalid path %q: %w",
					failureMark(), crdName, name, specPath, err,
				)
			}

			// 3. Check for duplicate spec paths
			if seenPaths[specPath] {
				return errDuplicateServeFieldPath(crdName, specPath, name)
			}
			seenPaths[specPath] = true

			// 4. If path is nested, warn that validation against the CRD
			//    schema is not yet implemented (to be added later)
			if isNestedPath(specPath) {
				crd.Warnings.AddWarning(fmt.Sprintf(
					"CRD %q: serve.fields %q has nested path %q — "+
						"schema validation is not yet available. Verify the path exists "+
						"in the CRD spec (e.g., 'spec.%s') and that it is a leaf field.",
					crdName, name, specPath, specPath,
				))
			}
		}
	}
	return nil
}

func errDuplicateServeFieldPath(crd, path, field string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s Duplicate serve.fields path: %q
   CRD: %s
   Used by field: %q

Each serve.fields entry must have a unique spec path — two fields cannot map
to the same location in the CRD spec.

If you need two fields to represent the same spec path, consider using a
single field with an enum or conditional logic.
──────────────────────────────────────────────`, failureMark(), path, crd, field)
}
