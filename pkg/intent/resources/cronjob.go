package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/resources/core/cronjobs"
)

// CronJobFields are the fields accepted in the remote reconciler
// intent form for a cronjob resource.
type CronJobFields struct {
	Name              string            `json:"name"`
	Image             string            `json:"image"`
	Schedule          string            `json:"schedule"`
	Command           []string          `json:"command"`
	Args              []string          `json:"args"`
	ConcurrencyPolicy string            `json:"concurrencyPolicy"`
	Suspend           bool              `json:"suspend"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
}

// BuildCronJob builds the CronJob a "cronjob" resource intent describes, owned
// by owner. Required: name, image, schedule.
func BuildCronJob(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f CronJobFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("cronjob intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("cronjob intent: name is required")
	}
	if f.Image == "" {
		return nil, fmt.Errorf("cronjob intent: image is required")
	}
	if f.Schedule == "" {
		return nil, fmt.Errorf("cronjob intent: schedule is required")
	}

	namespace := resolveNamespace(owner, "")

	spec := cronjobs.Spec{
		Name:      f.Name,
		Namespace: namespace,
		Image:     f.Image,
		Schedule:  f.Schedule,
		Command:   f.Command,
		Args:      f.Args,
		Suspend:   f.Suspend,
	}

	if f.ConcurrencyPolicy != "" {
		spec.ConcurrencyPolicy = resolveConcurrencyPolicy(f.ConcurrencyPolicy)
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	obj := cronjobs.Build(owner, spec)

	rawMap, err := toObjectMap(obj, "batch/v1", "CronJob")
	if err != nil {
		return nil, fmt.Errorf("cronjob intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("cronjob intent: %w", err)
	}
	return rawMap, nil
}
