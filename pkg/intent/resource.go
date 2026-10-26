package intent

import (
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/intent/resources"
)

// BuildResource builds the built-in object a resource intent describes, such
// as {"type": "deployment", "name": …, "image": …}, owned by owner. Build is
// BuildCR is the same for a CR of a served CRD.
func BuildResource(typ string, fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	return resources.Build(typ, fields, owner)
}

// ResourceTypes returns the types a resource intent can name, sorted.
func ResourceTypes() []string { return resources.Types() }
