// Package crds checks spec.crds entries: unique names and kinds, dependsOn, and entry labels and annotations.
package crds

import (
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate/internal/check"
	"github.com/intentd/inrun/pkg/config"
)

type checker struct{ check.Checker }

// Check runs this block's checks in order and returns the first error.
func Check(cat *catalog.Catalog, cfg *config.Config) error {
	c := &checker{check.New(cat, cfg)}
	return check.First(
		c.validateUniqueness,
		c.validateDependsOn,
		c.validateCRDEntryMetadata,
	)
}

// DetectCycles reports a dependsOn cycle between CRDs.
func DetectCycles(cat *catalog.Catalog) error {
	return (&checker{check.New(cat, nil)}).detectDependencyCycles()
}
