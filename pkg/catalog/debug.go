package catalog

import (
	"github.com/intentd/inrun/pkg/logger"
)

// Debug catalog information from merger
func (k *Catalog) DebugCatalogInformation() {
	// [DEBUG] Contents of k.Security
	logger.Debug().Interface("catalog security", k.Security).Msg("catalog security")

	// [DEBUG] Contents of k.Spec
	logger.Debug().Interface("catalog spec", k.Spec).Msg("catalog spec")

	// [DEBUG] Contents of k.enabledCRDs
	logger.Debug().Interface("catalog enabledCRDs", k.enabledCRDs).Msg("catalog enabledCRDs")

	// [DEBUG] Contents of k.metadata
	logger.Debug().Interface("catalog metadata", k.metadata).Msg("catalog metadata")
}
