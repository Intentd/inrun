package deployments

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/resources/shared"
	corev1 "k8s.io/api/core/v1"
)

// Build converts a flat intent fields map into a full Deployment object map
// suitable for inclusion in domain.Result.Resources and application via SSA.
func BuildFromIntent(fields map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f shared.DeploymentIntentFields
	if err := shared.DecodeFields(fields, &f); err != nil {
		return nil, fmt.Errorf("deployment.BuildFromIntent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("deployment.BuildFromIntent: name is required")
	}
	if f.Image == "" {
		return nil, fmt.Errorf("deployment.BuildFromIntent: image is required")
	}

	replicas := f.Replicas
	if replicas == 0 {
		replicas = 1
	}
	namespace := shared.ResolveNamespace(owner, "")

	spec := ResolvedDeploymentSpec{
		Name:      f.Name,
		Namespace: namespace,
		Image:     f.Image,
		Replicas:  replicas,
		Port:      f.Port,
		Protocol:  corev1.ProtocolTCP,
		Command:   f.Command,
	}

	obj := buildDeployment(owner, spec, namespace)

	for k, v := range f.Env {
		obj.Spec.Template.Spec.Containers[0].Env = append(
			obj.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{Name: k, Value: v},
		)
	}

	rawMap, err := shared.ToObjectMap(obj, "apps/v1", "Deployment")
	if err != nil {
		return nil, fmt.Errorf("deployment.BuildFromIntent: convert to unstructured: %w", err)
	}
	if err := shared.ApplyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("deployment.BuildFromIntent: %w", err)
	}
	return rawMap, nil
}
