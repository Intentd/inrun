package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/resources/core/configmaps"
)

// ConfigMapFields are the fields accepted in the remote reconciler
// intent form for a configmap resource.
type ConfigMapFields struct {
	Name        string            `json:"name"`
	Data        map[string]string `json:"data"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// BuildConfigMap builds the ConfigMap a "configmap" resource intent describes, owned
// by owner. Required: name, data.
func BuildConfigMap(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f ConfigMapFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("configmap intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("configmap intent: name is required")
	}
	if len(f.Data) == 0 {
		return nil, fmt.Errorf("configmap intent: data is required")
	}

	namespace := resolveNamespace(owner, "")
	spec := configmaps.Spec{Name: f.Name, Namespace: namespace}

	spec.Data = f.Data
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	obj := configmaps.Build(owner, spec)
	rawMap, err := toObjectMap(obj, "v1", "ConfigMap")
	if err != nil {
		return nil, fmt.Errorf("configmap intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("configmap intent: %w", err)
	}
	return rawMap, nil
}
