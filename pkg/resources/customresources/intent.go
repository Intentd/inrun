package customresources

import (
	"fmt"

	"github.com/intentd/inrun/pkg/util/kube"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
	"github.com/intentd/inrun/pkg/types"
)

// BuildFromIntent converts a flat intent fields map into a full custom resource
// object map suitable for inclusion in domain.Result.Resources and application via SSA.
// Required fields: apiVersion, kind, name. Namespace falls back to the owner's namespace.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.CustomResourceIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("custom.BuildFromIntent: %w", err)
	}
	if f.APIVersion == "" {
		return nil, fmt.Errorf("custom.BuildFromIntent: apiVersion is required")
	}
	if f.Kind == "" {
		return nil, fmt.Errorf("custom.BuildFromIntent: kind is required")
	}
	if f.Name == "" {
		return nil, fmt.Errorf("custom.BuildFromIntent: name is required")
	}

	gvk, err := kube.GVKFromFields(f.APIVersion, f.Kind)
	if err != nil {
		return nil, fmt.Errorf("custom.BuildFromIntent: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, f.Namespace)

	spec := ResolvedCustomResourceSpec{
		APIVersion: f.APIVersion,
		Kind:       f.Kind,
		Metadata:   types.CustomResourceMetadata{Name: f.Name},
		Spec:       f.Spec,
	}

	u := buildUnstructured(spec, owner, gvk, namespace)
	if err := shared.ApplyMetaOverrides(u.Object, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("custom.BuildFromIntent: %w", err)
	}
	return u.Object, nil
}
