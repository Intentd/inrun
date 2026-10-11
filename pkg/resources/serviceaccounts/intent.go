package serviceaccounts

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
)

// Build converts a flat intent fields map into a full ServiceAccount object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.ServiceAccountIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("serviceaccount.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("serviceaccount.BuildFromIntent: name is required")
	}

	namespace := shared.ResolveNamespace(owner, "")
	spec := ResolvedServiceAccountSpec{Name: f.Name, Namespace: namespace}

	obj := buildServiceAccount(owner, spec, namespace)
	rawMap, err := shared.ToObjectMap(obj, "v1", "ServiceAccount")
	if err != nil {
		return nil, fmt.Errorf("serviceaccount.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("serviceaccount.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
