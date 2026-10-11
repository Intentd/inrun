package statefulsets

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
	corev1 "k8s.io/api/core/v1"
)

// BuildFromIntent converts a flat intent fields map into a full StatefulSet object map
// suitable for application via SSA.
// Required fields: name, image.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.StatefulSetIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("statefulset.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("statefulset.BuildFromIntent: name is required")
	}
	if f.Image == "" {
		return nil, fmt.Errorf("statefulset.BuildFromIntent: image is required")
	}

	replicas := f.Replicas
	if replicas == 0 {
		replicas = 1
	}
	namespace := shared.ResolveNamespace(owner, "")

	spec := ResolvedStatefulSetSpec{
		Name:        f.Name,
		Namespace:   namespace,
		Image:       f.Image,
		Replicas:    replicas,
		Port:        f.Port,
		Protocol:    corev1.ProtocolTCP,
		ServiceName: f.ServiceName,
	}

	obj := buildStatefulSet(owner, spec, namespace)

	for k, v := range f.Env {
		obj.Spec.Template.Spec.Containers[0].Env = append(
			obj.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{Name: k, Value: v},
		)
	}

	rawMap, err := shared.ToObjectMap(obj, "apps/v1", "StatefulSet")
	if err != nil {
		return nil, fmt.Errorf("statefulset.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("statefulset.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
