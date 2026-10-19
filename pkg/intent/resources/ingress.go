package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/resources/core/ingresses"
)

// IngressFields are the fields accepted in the remote reconciler
// intent form for an ingress resource.
type IngressFields struct {
	Name         string            `json:"name"`
	Host         string            `json:"host"`
	ServiceName  string            `json:"serviceName"`
	ServicePort  int32             `json:"servicePort"`
	Path         string            `json:"path"`
	PathType     string            `json:"pathType"`
	IngressClass string            `json:"ingressClass"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

// BuildIngress builds the Ingress a "ingress" resource intent describes, owned
// by owner. Required: name, serviceName, servicePort.
func BuildIngress(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f IngressFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("ingress intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("ingress intent: name is required")
	}
	if f.ServiceName == "" {
		return nil, fmt.Errorf("ingress intent: serviceName is required")
	}
	if f.ServicePort == 0 {
		return nil, fmt.Errorf("ingress intent: servicePort is required")
	}

	path := f.Path
	if path == "" {
		path = "/"
	}

	namespace := resolveNamespace(owner, "")

	spec := ingresses.Spec{
		Name:         f.Name,
		Namespace:    namespace,
		Host:         f.Host,
		ServiceName:  f.ServiceName,
		ServicePort:  f.ServicePort,
		Path:         path,
		PathType:     f.PathType,
		IngressClass: f.IngressClass,
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	obj := ingresses.Build(owner, spec)

	rawMap, err := toObjectMap(obj, "networking.k8s.io/v1", "Ingress")
	if err != nil {
		return nil, fmt.Errorf("ingress intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("ingress intent: %w", err)
	}
	return rawMap, nil
}
