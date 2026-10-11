package services

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
)

// Build converts a flat intent fields map into a full Service object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.ServiceIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("service.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("service.BuildFromIntent: name is required")
	}
	if f.Port == 0 {
		return nil, fmt.Errorf("service.BuildFromIntent: port is required")
	}

	targetPort := f.TargetPort
	if targetPort == 0 {
		targetPort = f.Port
	}

	namespace := shared.ResolveNamespace(owner, "")
	spec := ResolvedServiceSpec{
		Name:       f.Name,
		Namespace:  namespace,
		Port:       f.Port,
		TargetPort: targetPort,
		Type:       f.Type,
		Selector:   make(map[string]string),
	}

	obj := buildService(owner, spec, namespace)
	rawMap, err := shared.ToObjectMap(obj, "v1", "Service")
	if err != nil {
		return nil, fmt.Errorf("service.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("service.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
