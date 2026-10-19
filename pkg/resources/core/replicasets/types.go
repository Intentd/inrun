// pkg/resources/replicasets/types.go
package replicasets

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"
)

// Spec is the fully resolved ReplicaSet specification.
// Produced by resolving template expressions and merging static values.
// Passed directly to Create, Apply and Delete.
type Spec struct {
	// Name — resolved ReplicaSet name. Required.
	Name string

	// Image — container image. Required.
	Image string

	// Replicas — number of pod replicas. Default: 1.
	Replicas int32

	// Port — container port. 0 means no port exposed.
	Port     int32
	Protocol corev1.Protocol

	// Namespace — target namespace. Required.
	Namespace string

	// Labels — applied to the ReplicaSet and pod template.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels map[string]string

	// Annotations — applied to the ReplicaSet.
	Annotations map[string]string

	// Env — environment variables.
	Env     []types.EnvVar
	EnvFrom *types.EnvFrom

	// Resources — CPU and memory requests/limits. nil means no limits set.
	Resources *types.ResourceRequirements

	// Selector is the label selector for the pods, set on the pod template too.
	// Empty means app.kubernetes.io/name: Name.
	Selector map[string]string

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

	// RollingUpdate — resolved rolling update strategy. nil means Inrun manages lifecycle directly.
	RollingUpdate *types.RollingUpdateBehavior

	// Volumes / VolumeMounts — pod volumes and container mounts.
	Volumes      []types.VolumeSource
	VolumeMounts []types.VolumeMount
}
