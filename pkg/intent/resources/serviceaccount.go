package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/resources/core/serviceaccounts"
)

// ServiceAccountFields are the fields accepted in the remote reconciler
// intent form for a serviceaccount resource.
type ServiceAccountFields struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// BuildServiceAccount builds the ServiceAccount a "serviceaccount" resource intent describes, owned
// by owner. Required: name.
func BuildServiceAccount(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f ServiceAccountFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("serviceaccount intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("serviceaccount intent: name is required")
	}

	namespace := resolveNamespace(owner, "")
	spec := serviceaccounts.Spec{Name: f.Name, Namespace: namespace}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	obj := serviceaccounts.Build(owner, spec)
	rawMap, err := toObjectMap(obj, "v1", "ServiceAccount")
	if err != nil {
		return nil, fmt.Errorf("serviceaccount intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("serviceaccount intent: %w", err)
	}
	return rawMap, nil
}
