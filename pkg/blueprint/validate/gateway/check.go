// Package gateway checks gateway: the features that need a gateway, tokens, and clusters.
package gateway

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
		c.validateGateway,
		c.validateGatewayTokens,
		c.validateGatewayClusters,
	)
}

// Clusters checks gateway.clusters and the serve targets that reference them.
func Clusters(bp *blueprint.Blueprint) error {
	return (&checker{check.New(bp, nil)}).validateGatewayClusters()
}
