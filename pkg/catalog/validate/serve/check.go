// Package serve checks serve: the Gateway API surface of each CRD.
package serve

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
		c.validateServe,
	)
}

// Validate checks every CRD's serve configuration on its own.
func Validate(cat *catalog.Catalog) error { return (&checker{check.New(cat, nil)}).validateServe() }
