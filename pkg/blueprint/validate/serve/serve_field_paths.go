package serve

import (
	"fmt"
	"strings"
)

// validateServeFieldPaths validates serve.fields path configurations:
//   - Paths (if set) must be unique across all fields
//   - Path must be a valid dot-notation path (no empty segments, no leading/trailing dots)
//   - Warn if path is nested (contains a dot) — schema existence validation is not yet implemented
func (c *checker) validateServeFieldPaths() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		if !crd.ServeEnabled() {
			continue
		}

		if !crd.HasServeFields() {
			continue
		}

		seenPaths := make(map[string]bool)
		seenMigrateFrom := make(map[string]string) // from-path → field name that claimed it

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

		for name, config := range crd.Serve.Fields {
			if !config.HasMigrate() {
				continue
			}
			from := strings.TrimPrefix(config.MigrateFrom(), "spec.")

			// 5. Duplicate migrate.from: two fields reading from the same old path.
			//    The second field would silently skip on every run — the first
			//    deletes the source key before the second gets a chance to read it.
			if prior, ok := seenMigrateFrom[from]; ok {
				return errDuplicateMigrateFrom(crdName, from, prior, name)
			}

			// 6. Overlapping migrate.from paths: one path is an ancestor of another.
			//    prepareField() deletes the source key, so if "foo" is migrated before
			//    "foo.bar", the child key is gone; if "foo.bar" runs first, the parent
			//    read returns an incomplete object. Neither order is correct — reject.
			for prior, priorField := range seenMigrateFrom {
				if strings.HasPrefix(from, prior+".") || strings.HasPrefix(prior, from+".") {
					return errOverlappingMigrateFrom(crdName, from, name, prior, priorField)
				}
			}

			seenMigrateFrom[from] = name
		}

		// Write the modified copy (warnings) back into the blueprint map.
		c.Blueprint.EnabledCRDs()[crdName] = crd
	}
	return nil
}

func errOverlappingMigrateFrom(crd, from, field, prior, priorField string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s Overlapping migrate.from paths: %q and %q
   CRD: %s
   Fields: %q and %q

One migrate.from path is an ancestor of the other. The migrator deletes the
source key after reading it, so whichever field runs first corrupts the other:
migrating the parent removes the child's source; migrating the child first
leaves the parent with an incomplete object.

Declare non-overlapping source paths, or consolidate into a single field.
──────────────────────────────────────────────`, failureMark(), from, prior, crd, field, priorField)
}

func errDuplicateMigrateFrom(crd, from, first, second string) error {
	return fmt.Errorf(`
──────────────────────────────────────────────
%s Duplicate migrate.from path: %q
   CRD: %s
   Claimed by: %q and %q

Two serve fields cannot read from the same migrate.from path. The migrator
deletes the source key after the first field reads it — the second field
would find the key absent and silently skip on every run.

Use a different source path, or consolidate into one field with a values:
transform that writes to multiple destinations.
──────────────────────────────────────────────`, failureMark(), from, crd, first, second)
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
