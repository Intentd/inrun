package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/core/namespaces"
)

// NamespaceFields are the fields accepted in the remote reconciler
// intent form for a namespace resource.
type NamespaceFields struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// BuildNamespace builds the Namespace a "namespace" resource intent describes, owned
// by owner. Required: name.
func BuildNamespace(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f NamespaceFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("namespace intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("namespace intent: name is required")
	}

	spec := namespaces.Spec{Name: f.Name}

	obj := namespaces.Build(owner, spec)
	rawMap, err := toObjectMap(obj, "v1", "Namespace")
	if err != nil {
		return nil, fmt.Errorf("namespace intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("namespace intent: %w", err)
	}
	return rawMap, nil
}
