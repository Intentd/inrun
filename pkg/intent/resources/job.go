package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/kubewright/core/jobs"
)

// JobFields are the fields accepted in the remote reconciler
// intent form for a job resource.
type JobFields struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command"`
	Args         []string          `json:"args"`
	BackoffLimit int               `json:"backoffLimit"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

// BuildJob builds the Job a "job" resource intent describes, owned
// by owner. Required: name, image.
func BuildJob(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f JobFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("job intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("job intent: name is required")
	}
	if f.Image == "" {
		return nil, fmt.Errorf("job intent: image is required")
	}

	backoffLimit := f.BackoffLimit
	if backoffLimit == 0 {
		backoffLimit = 3
	}

	namespace := resolveNamespace(owner, "")

	spec := jobs.Spec{
		Name:         f.Name,
		Namespace:    namespace,
		Image:        f.Image,
		Command:      f.Command,
		Args:         f.Args,
		BackoffLimit: backoffLimit,
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	obj := jobs.Build(owner, spec)

	rawMap, err := toObjectMap(obj, "batch/v1", "Job")
	if err != nil {
		return nil, fmt.Errorf("job intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("job intent: %w", err)
	}
	return rawMap, nil
}
