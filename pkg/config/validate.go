package config

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

// -----------------------------------------------------------------------------
func Validate() *validator.Validate {
	validate := validator.New(validator.WithRequiredStructEnabled())
	return validate
}

// Normalize environment
func (c *Config) normalizeEnvironment() {
	// Normalize inrun environment
	switch strings.ToLower(c.inrun.environment) {
	case DevShort, Development:
		c.inrun.environment = Development
	case StagingShort, Staging:
		c.inrun.environment = Staging
	case Live, ProdShort, Production:
		c.inrun.environment = Production
	default:
		c.inrun.environment = Development
	}
}
