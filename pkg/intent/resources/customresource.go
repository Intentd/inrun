package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/kube"
	"github.com/intentd/kubekit/core/customresources"
)

// CustomResourceFields are the fields accepted in the remote reconciler
// intent form for a custom resource (type: custom).
// The spec is schema-agnostic; any top-level fields beyond spec must use full form.
type CustomResourceFields struct {
	APIVersion  string                 `json:"apiVersion"`
	Kind        string                 `json:"kind"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace,omitempty"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
}

// BuildCustomResource builds the custom resource a "custom" resource intent describes, owned
// by owner. Required: apiVersion, kind, name.
func BuildCustomResource(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f CustomResourceFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("custom intent: %w", err)
	}
	if f.APIVersion == "" {
		return nil, fmt.Errorf("custom intent: apiVersion is required")
	}
	if f.Kind == "" {
		return nil, fmt.Errorf("custom intent: kind is required")
	}
	if f.Name == "" {
		return nil, fmt.Errorf("custom intent: name is required")
	}

	if _, err := kube.GVKFromFields(f.APIVersion, f.Kind); err != nil {
		return nil, fmt.Errorf("custom intent: %w", err)
	}

	spec := customresources.Spec{
		APIVersion: f.APIVersion,
		Kind:       f.Kind,
		Metadata:   types.CustomResourceMetadata{Name: f.Name, Namespace: f.Namespace},
		Spec:       f.Spec,
	}

	spec.Metadata.Labels = labels.StampInrunLabels(spec.Metadata.Labels, owner.GetName(), owner.GetAnnotations())
	u := customresources.Build(owner, spec)
	if err := applyMetaOverrides(u.Object, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("custom intent: %w", err)
	}
	return u.Object, nil
}
