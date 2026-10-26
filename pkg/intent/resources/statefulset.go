package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	tmplresources "github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/kubewright/core/statefulsets"
	kwtypes "github.com/intentd/kubewright/core/types"
)

// StatefulSetFields are the fields accepted in the remote reconciler
// intent form for a statefulset resource.
type StatefulSetFields struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int32             `json:"replicas"`
	Port        int32             `json:"port"`
	Protocol    string            `json:"protocol"` // TCP (default), UDP or SCTP
	ServiceName string            `json:"serviceName"`
	Env         map[string]string `json:"env"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	// Containers and InitContainers describe a pod with more than one
	// container: image and port then stay empty, and env is inherited by
	// every container.
	Containers     []kwtypes.Container `json:"containers"`
	InitContainers []kwtypes.Container `json:"initContainers"`
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
	if err := checkImage("statefulset", f.Image, f.Containers); err != nil {
		return nil, err
	}

	replicas := f.Replicas
	if replicas == 0 {
		replicas = 1
	}
	namespace := resolveNamespace(owner, "")

	spec := statefulsets.Spec{
		Name:           f.Name,
		Namespace:      namespace,
		Image:          f.Image,
		Replicas:       replicas,
		Port:           f.Port,
		Protocol:       tmplresources.ParseProtocol(f.Protocol),
		ServiceName:    f.ServiceName,
		Env:            envVars(f.Env),
		Containers:     f.Containers,
		InitContainers: f.InitContainers,
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	spec.Selector = map[string]string{labels.InrunOwner: owner.GetName()}
	obj := statefulsets.Build(owner, spec)

	rawMap, err := toObjectMap(obj, "apps/v1", "StatefulSet")
	if err != nil {
		return nil, fmt.Errorf("statefulset intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("statefulset intent: %w", err)
	}
	return rawMap, nil
}
