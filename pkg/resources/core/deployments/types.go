// pkg/resources/deployments/types.go
package deployments

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/pkg/resources/presets"

	restypes "github.com/intentd/inrun/pkg/resources/core/types"
)

// Spec is the fully resolved Deployment specification.
// Produced by resolving template expressions and merging static values.
// Passed directly to Create, Apply and Delete.
type Spec struct {
	// Name — resolved Deployment name. Required.
	Name string

	// Image — container image. Required.
	Image string

	// Replicas — number of pod replicas. Default: 1.
	Replicas int32

	// HasAutoscale — when true, the workload autoscaler owns spec.replicas.
	// The drift check skips replicas so the reconciler does not fight the autoscaler.
	HasAutoscale bool

	// Port — container port. 0 means no port exposed.
	Port int32

	// Protocol — resolved container port protocol. Defaults to TCP when not declared.
	Protocol corev1.Protocol

	// Namespace — target namespace. Required.
	Namespace string

	// Labels — applied to Deployment and pod template.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels map[string]string

	// Annotations — applied to the Deployment.
	Annotations map[string]string

	// Env — environment variables.
	Env     []restypes.EnvVar
	EnvFrom *restypes.EnvFrom

	// Resources — CPU and memory requests/limits. nil means no limits set.
	Resources *restypes.ResourceRequirements

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
	Probes *restypes.ProbesConfig

	// SecurityContext — container-level security settings.
	SecurityContext *restypes.ContainerSecurityContext

	// PodSecurity — pod-level security settings.
	PodSecurity *restypes.PodSecurityContext

	// Presets — the presets to look probe timings up in: built-ins merged with your own.
	Presets presets.Registry

	// RollingUpdate — resolved rolling update strategy.
	// nil means use Kubernetes defaults (25%/25%).
	RollingUpdate *restypes.RollingUpdateBehavior

	// Command overrides the container entrypoint command.
	Command []string

	// Volumes / VolumeMounts — pod volumes and container mounts.
	Volumes      []restypes.VolumeSource
	VolumeMounts []restypes.VolumeMount

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}
