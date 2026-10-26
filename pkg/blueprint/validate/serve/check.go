// Package serve checks serve: the Gateway API surface of each CRD.
package serve

import (
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/blueprint/validate/internal/check"
	"github.com/intentd/inrun/pkg/config"
)

type checker struct{ check.Checker }

// Check runs this block's checks in order and returns the first error.
func Check(bp *blueprint.Blueprint, cfg *config.Config) error {
	c := &checker{check.New(bp, cfg)}
	return check.First(
		c.validateServe,
	)
}

// Validate checks every CRD's serve configuration on its own.
func Validate(bp *blueprint.Blueprint) error { return (&checker{check.New(bp, nil)}).validateServe() }
