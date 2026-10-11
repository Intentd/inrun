// Package gateway checks gateway: the features that need a gateway, tokens, and clusters.
package gateway

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
		c.validateGateway,
		c.validateGatewayTokens,
		c.validateGatewayClusters,
	)
}

// Clusters checks gateway.clusters and the serve targets that reference them.
func Clusters(cat *catalog.Catalog) error {
	return (&checker{check.New(cat, nil)}).validateGatewayClusters()
}
