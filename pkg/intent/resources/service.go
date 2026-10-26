package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/kubewright/core/services"
)

// ServiceFields are the fields accepted in the remote reconciler
// intent form for a service resource.
type ServiceFields struct {
	Name        string            `json:"name"`
	Port        int32             `json:"port"`
	TargetPort  int32             `json:"targetPort"`
	Protocol    string            `json:"protocol"` // TCP (default), UDP or SCTP
	Type        string            `json:"type"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// BuildService builds the Service a "service" resource intent describes, owned
// by owner. Required: name, port.
func BuildService(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f ServiceFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("service intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("service intent: name is required")
	}
	if f.Port == 0 {
		return nil, fmt.Errorf("service intent: port is required")
	}

	targetPort := f.TargetPort
	if targetPort == 0 {
		targetPort = f.Port
	}

	namespace := resolveNamespace(owner, "")
	spec := services.Spec{
		Name:       f.Name,
		Namespace:  namespace,
		Port:       f.Port,
		TargetPort: targetPort,
		Protocol:   f.Protocol,
		Type:       f.Type,
		Selector:   map[string]string{labels.InrunOwner: owner.GetName()},
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	obj := services.Build(owner, spec)
	rawMap, err := toObjectMap(obj, "v1", "Service")
	if err != nil {
		return nil, fmt.Errorf("service intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("service intent: %w", err)
	}
	return rawMap, nil
}
