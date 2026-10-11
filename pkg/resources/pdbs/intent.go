// pkg/resources/pdbs/pdb.go
package pdbs

import (
	"fmt"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/resources/shared"
)

// Build converts a flat intent fields map into a full PodDisruptionBudget object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.PDBIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("pdb.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("pdb.BuildFromIntent: name is required")
	}

	namespace := shared.ResolveNamespace(owner, "")
	spec := ResolvedPDBSpec{
		Name:           f.Name,
		Namespace:      namespace,
		Selector:       f.Selector,
		MinAvailable:   f.MinAvailable,
		MaxUnavailable: f.MaxUnavailable,
	}

	obj := buildPDB(owner, spec, namespace)
	rawMap, err := shared.ToObjectMap(obj, "policy/v1", "PodDisruptionBudget")
	if err != nil {
		return nil, fmt.Errorf("pdb.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("pdb.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
