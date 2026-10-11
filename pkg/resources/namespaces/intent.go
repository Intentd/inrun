// pkg/resources/namespace/intent.go
package namespaces

import (
	"fmt"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/resources/shared"
)

// Build converts a flat intent fields map into a full Namespace object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.NamespaceIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("namespace.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("namespace.BuildFromIntent: name is required")
	}

	spec := ResolvedNamespaceSpec{Name: f.Name}

	obj := buildNamespace(owner, spec)
	rawMap, err := shared.ToObjectMap(obj, "v1", "Namespace")
	if err != nil {
		return nil, fmt.Errorf("namespace.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("namespace.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
