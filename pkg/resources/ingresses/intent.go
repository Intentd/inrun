package ingresses

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
)

// BuildFromIntent converts a flat intent fields map into a full Ingress object map
// suitable for application via SSA.
// Required fields: name, serviceName, servicePort.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.IngressIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("ingress.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("ingress.BuildFromIntent: name is required")
	}
	if f.ServiceName == "" {
		return nil, fmt.Errorf("ingress.BuildFromIntent: serviceName is required")
	}
	if f.ServicePort == 0 {
		return nil, fmt.Errorf("ingress.BuildFromIntent: servicePort is required")
	}

	path := f.Path
	if path == "" {
		path = "/"
	}

	namespace := shared.ResolveNamespace(owner, "")

	spec := ResolvedIngressSpec{
		Name:         f.Name,
		Namespace:    namespace,
		Host:         f.Host,
		ServiceName:  f.ServiceName,
		ServicePort:  f.ServicePort,
		Path:         path,
		PathType:     f.PathType,
		IngressClass: f.IngressClass,
	}

	obj := buildIngress(owner, spec, namespace)

	rawMap, err := shared.ToObjectMap(obj, "networking.k8s.io/v1", "Ingress")
	if err != nil {
		return nil, fmt.Errorf("ingress.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("ingress.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
