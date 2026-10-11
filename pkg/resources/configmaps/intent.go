package configmaps

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
)

// BuildFromIntent converts a flat intent fields map into a full ConfigMap object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.ConfigMapIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("configmap.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("configmap.BuildFromIntent: name is required")
	}
	if len(f.Data) == 0 {
		return nil, fmt.Errorf("configmap.BuildFromIntent: data is required")
	}

	namespace := shared.ResolveNamespace(owner, "")
	spec := ResolvedConfigMapSpec{Name: f.Name, Namespace: namespace}

	obj := buildConfigMap(owner, spec, namespace, f.Data)
	rawMap, err := shared.ToObjectMap(obj, "v1", "ConfigMap")
	if err != nil {
		return nil, fmt.Errorf("configmap.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("configmap.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
