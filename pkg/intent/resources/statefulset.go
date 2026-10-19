package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/kubekit/core/statefulsets"
)

// StatefulSetFields are the fields accepted in the remote reconciler
// intent form for a statefulset resource.
type StatefulSetFields struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int32             `json:"replicas"`
	Port        int32             `json:"port"`
	ServiceName string            `json:"serviceName"`
	Env         map[string]string `json:"env"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// BuildStatefulSet builds the StatefulSet a "statefulset" resource intent describes, owned
// by owner. Required: name, image.
func BuildStatefulSet(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f StatefulSetFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("statefulset intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("statefulset intent: name is required")
	}
	if f.Image == "" {
		return nil, fmt.Errorf("statefulset intent: image is required")
	}

	replicas := f.Replicas
	if replicas == 0 {
		replicas = 1
	}
	namespace := resolveNamespace(owner, "")

	spec := statefulsets.Spec{
		Name:        f.Name,
		Namespace:   namespace,
		Image:       f.Image,
		Replicas:    replicas,
		Port:        f.Port,
		Protocol:    corev1.ProtocolTCP,
		ServiceName: f.ServiceName,
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	spec.Selector = map[string]string{labels.InrunOwner: owner.GetName()}
	obj := statefulsets.Build(owner, spec)

	for k, v := range f.Env {
		obj.Spec.Template.Spec.Containers[0].Env = append(
			obj.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{Name: k, Value: v},
		)
	}

	rawMap, err := toObjectMap(obj, "apps/v1", "StatefulSet")
	if err != nil {
		return nil, fmt.Errorf("statefulset intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("statefulset intent: %w", err)
	}
	return rawMap, nil
}
