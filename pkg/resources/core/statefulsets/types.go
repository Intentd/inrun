// pkg/resources/statefulsets/types.go
package statefulsets

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"
)

// VolumeClaimTemplate is one resolved PVC template for a StatefulSet.
type VolumeClaimTemplate struct {
	Name         string
	StorageClass string
	StorageSize  string
	MountPath    string
	AccessModes  []string
}

// Spec is the fully resolved StatefulSet specification.
type Spec struct {
	Name         string
	Namespace    string
	Image        string
	Replicas     int32
	HasAutoscale bool
	Port         int32
	Protocol     corev1.Protocol
	ServiceName  string

	// VolumeClaimTemplates — resolved PVC templates (may be multiple).
	VolumeClaimTemplates []VolumeClaimTemplate

	Labels      map[string]string
	Annotations map[string]string

	Env       []types.EnvVar
	EnvFrom   *types.EnvFrom
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

	VolumeClaimRetentionPolicy VolumeClaimRetentionPolicy

	// Probes — startup, liveness, and readiness probe configuration.
	Probes *types.ProbesConfig

	// SecurityContext — container-level security settings.
	SecurityContext *types.ContainerSecurityContext

	// PodSecurity — pod-level security settings.
	PodSecurity *types.PodSecurityContext

	// Presets — the presets to look probe timings up in: built-ins merged with your own.
	Presets presets.Registry

	// RollingUpdate — resolved rolling update strategy. nil uses OnDelete (Inrun default).
	RollingUpdate *types.RollingUpdateBehavior

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

// VolumeClaimRetentionPolicy describes the policy used for PVCs
// created from the StatefulSet VolumeClaimTemplates.
type VolumeClaimRetentionPolicy struct {
	// WhenDeleted specifies what happens to PVCs created from StatefulSet
	// VolumeClaimTemplates when the StatefulSet is deleted. The default policy
	// of `Retain` causes PVCs to not be affected by StatefulSet deletion. The
	// `Delete` policy causes those PVCs to be deleted.
	WhenDeleted string
	// WhenScaled specifies what happens to PVCs created from StatefulSet
	// VolumeClaimTemplates when the StatefulSet is scaled down. The default
	// policy of `Retain` causes PVCs to not be affected by a scaledown. The
	// `Delete` policy causes the associated PVCs for any excess pods above
	// the replica count to be deleted.
	WhenScaled string
}
