package catalog

import (
	"github.com/intentd/inrun/pkg/logger"
)

// Debug catalog information from merger
func (c *Catalog) DebugCatalogInformation() {
	// [DEBUG] Contents of k.Security
	logger.Debug().Interface("catalog security", c.Security).Msg("catalog security")

	// [DEBUG] Contents of k.Spec
	logger.Debug().Interface("catalog spec", c.Spec).Msg("catalog spec")

	// [DEBUG] Contents of k.enabledCRDs
	logger.Debug().Interface("catalog enabledCRDs", c.enabledCRDs).Msg("catalog enabledCRDs")

	// [DEBUG] Contents of k.metadata
	logger.Debug().Interface("catalog metadata", c.metadata).Msg("catalog metadata")
}
