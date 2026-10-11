package cronjobs

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
	batchv1 "k8s.io/api/batch/v1"
)

// BuildFromIntent converts a flat intent fields map into a full CronJob object map
// suitable for application via SSA.
// Required fields: name, image, schedule.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.CronJobIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("cronjob.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("cronjob.BuildFromIntent: name is required")
	}
	if f.Image == "" {
		return nil, fmt.Errorf("cronjob.BuildFromIntent: image is required")
	}
	if f.Schedule == "" {
		return nil, fmt.Errorf("cronjob.BuildFromIntent: schedule is required")
	}

	namespace := shared.ResolveNamespace(owner, "")

	spec := ResolvedCronJobSpec{
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

	obj := buildCronJob(owner, spec, namespace)

	rawMap, err := shared.ToObjectMap(obj, "batch/v1", "CronJob")
	if err != nil {
		return nil, fmt.Errorf("cronjob.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("cronjob.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}

func resolveConcurrencyPolicy(s string) batchv1.ConcurrencyPolicy {
	switch strings.ToLower(s) {
	case "forbid":
		return batchv1.ForbidConcurrent
	case "replace":
		return batchv1.ReplaceConcurrent
	default:
		return batchv1.AllowConcurrent
	}
}
