package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/kuberesources/core/pdbs"
)

// PDBFields  are the fields accepted in the remote reconciler
// intent form for a PodDisruptionBudget resource.
type PDBFields struct {
	Name           string            `json:"name"`
	Selector       map[string]string `json:"selector"`
	MinAvailable   string            `json:"minAvailable"`
	MaxUnavailable string            `json:"maxUnavailable"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
}

// BuildPDB builds the PodDisruptionBudget a "pdb" resource intent describes, owned
// by owner. Required: name.
func BuildPDB(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f PDBFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("pdb intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("pdb intent: name is required")
	}

	namespace := resolveNamespace(owner, "")
	spec := pdbs.Spec{
		Name:           f.Name,
		Namespace:      namespace,
		Selector:       f.Selector,
		MinAvailable:   f.MinAvailable,
		MaxUnavailable: f.MaxUnavailable,
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	obj := pdbs.Build(owner, spec)
	rawMap, err := toObjectMap(obj, "policy/v1", "PodDisruptionBudget")
	if err != nil {
		return nil, fmt.Errorf("pdb intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("pdb intent: %w", err)
	}
	return rawMap, nil
}
