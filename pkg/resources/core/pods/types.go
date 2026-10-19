// pkg/resources/pods/types.go
package pods

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"
)

// Spec is the fully resolved Pod specification.
// Produced by merging PodFromCRD (dynamic) and PodFromBlueprint (static).
// fromCRD wins over fromBlueprint when both declare the same field.
// Passed directly to Create, Apply and Delete.
type Spec struct {
	// Name — resolved Pod name. Required.
	Name string

	// Image — container image. Required.
	Image string

	// Namespace — target namespace. Required.
	Namespace string

	// Port — container port. 0 means no port exposed.
	Port     int
	Protocol corev1.Protocol

	// Labels — merged labels from both sources.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels map[string]string

	// Annotations — merged annotations from both sources.
	Annotations map[string]string

	// Resources — CPU and memory requests/limits. nil means no limits set.
	Resources *types.ResourceRequirements

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string

	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// +optional
	ServiceAccountName string

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string

	// Probes — startup, liveness, and readiness probe configuration.
	Probes *types.ProbesConfig

	// SecurityContext — container-level security settings.
	SecurityContext *types.ContainerSecurityContext

	// PodSecurity — pod-level security settings.
	PodSecurity *types.PodSecurityContext

	// Presets — the presets to look probe timings up in: built-ins merged with your own.
	Presets presets.Registry

	// Volumes / VolumeMounts — pod volumes and container mounts.
	Volumes      []types.VolumeSource
	VolumeMounts []types.VolumeMount

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}
