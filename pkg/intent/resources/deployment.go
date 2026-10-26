package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	tmplresources "github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/kubewright/core/deployments"
	kwtypes "github.com/intentd/kubewright/core/types"
)

// DeploymentFields are the fields accepted in the remote reconciler
// intent form for a deployment resource.
type DeploymentFields struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int32             `json:"replicas"`
	Port        int32             `json:"port"`
	Protocol    string            `json:"protocol"` // TCP (default), UDP or SCTP
	Env         map[string]string `json:"env"`
	Command     []string          `json:"command"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	// Containers and InitContainers describe a pod with more than one
	// container: image, command and port then stay empty, and env is
	// inherited by every container.
	Containers     []kwtypes.Container `json:"containers"`
	InitContainers []kwtypes.Container `json:"initContainers"`
}

// BuildDeployment builds the Deployment a "deployment" resource intent describes, owned
// by owner. Required: name, and image or containers.
func BuildDeployment(in map[string]interface{}, owner domain.Object) (map[string]interface{}, error) {
	var f DeploymentFields
	if err := decodeFields(in, &f); err != nil {
		return nil, fmt.Errorf("deployment intent: %w", err)
	}
	if f.Name == "" {
		return nil, fmt.Errorf("deployment intent: name is required")
	}
	if err := checkImage("deployment", f.Image, f.Containers); err != nil {
		return nil, err
	}

	replicas := f.Replicas
	if replicas == 0 {
		replicas = 1
	}
	namespace := resolveNamespace(owner, "")

	spec := deployments.Spec{
		Name:           f.Name,
		Namespace:      namespace,
		Image:          f.Image,
		Replicas:       replicas,
		Port:           f.Port,
		Protocol:       tmplresources.ParseProtocol(f.Protocol),
		Command:        f.Command,
		Env:            envVars(f.Env),
		Containers:     f.Containers,
		InitContainers: f.InitContainers,
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	spec.Selector = map[string]string{labels.InrunOwner: owner.GetName()}
	obj := deployments.Build(owner, spec)

	rawMap, err := toObjectMap(obj, "apps/v1", "Deployment")
	if err != nil {
		return nil, fmt.Errorf("deployment intent: convert to unstructured: %w", err)
	}
	if err := applyMetaOverrides(rawMap, f.Labels, f.Annotations); err != nil {
		return nil, fmt.Errorf("deployment intent: %w", err)
	}
	return rawMap, nil
}
