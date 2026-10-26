package emit

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/types"
)

// validateStatusConfig checks that all fields in a StatusConfig declare a valid type.
func validateStatusConfig(crdName, location string, sc *types.StatusConfig) error {
	if sc == nil || !sc.HasFields() {
		return nil
	}
	for _, f := range sc.Fields {
		switch strings.ToLower(f.Type) {
		case "", "string", "str", "default":
		case "int", "integer":
		case "bool", "boolean":
		case "float", "auto":
			// valid
		default:
			return fmt.Errorf(
				"%s invalid status field type %q in CRD %q (%s, path: %q):\n"+
					"  must be one of: string, str, int, integer, bool, boolean, float, auto\n",
				failureMark(), f.Type, crdName, location, f.Path,
			)
		}
	}
	return nil
}
