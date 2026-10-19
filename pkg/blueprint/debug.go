package blueprint

import (
	"github.com/intentd/inrun/pkg/logger"
)

// Debug blueprint information from merger
func (c *Blueprint) DebugBlueprintInformation() {
	// [DEBUG] Contents of k.Security
	logger.Debug().Interface("blueprint security", c.Security).Msg("blueprint security")

	// [DEBUG] Contents of k.Spec
	logger.Debug().Interface("blueprint spec", c.Spec).Msg("blueprint spec")

	// [DEBUG] Contents of k.enabledCRDs
	logger.Debug().Interface("blueprint enabledCRDs", c.enabledCRDs).Msg("blueprint enabledCRDs")

	// [DEBUG] Contents of k.metadata
	logger.Debug().Interface("blueprint metadata", c.metadata).Msg("blueprint metadata")
}
