package kubeclient

import (
	"github.com/intentd/inrun/pkg/config"
	restypes "github.com/intentd/inrun/pkg/resources/core/types"
)

// ApplyOptions returns how the runtime writes a child resource with
// server-side apply: as Inrun's field manager, forcing conflicts as the
// resource says (resourceForce), else as the CRD says, else true.
func ApplyOptions(c Interface, resourceForce *bool) restypes.Options {
	force := true
	switch {
	case resourceForce != nil:
		force = *resourceForce
	case c != nil && c.ForceConflict() != nil:
		force = *c.ForceConflict()
	}
	return restypes.Options{FieldManager: config.FieldManagerRuntime, Force: force}
}
