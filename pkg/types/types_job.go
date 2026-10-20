// pkg/types/types_job.go
package types

// ── Job ───────────────────────────────────────────────────────────────────────

// JobTemplateSource declares one Job to be run by Inrun.
//
// Most commonly used under onDelete for cleanup tasks that must complete
// before Inrun removes finalizers from the CR:
//   - Draining queues or buffers
//   - Archiving state to external storage
//   - Notifying external systems of deletion
//   - Running database migrations before removing a CRD instance
//
// Can also be used under onCreate for one-time provisioning tasks.
//
// Example (cleanup on delete):
//
//	onDelete:
//	  jobs:
//	    - name: "{{ .metadata.name }}-cleanup"
//	      image: busybox
//	      command: ["sh", "-c", "echo cleaning up {{ .metadata.name }}"]
//	      backoffLimit: 3
type JobTemplateSource struct {
	// Name — Job name.
	// Default when omitted: "{{ .metadata.name }}-job"
	Name string `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`

	// Image — container image. Required.
	Image string `yaml:"image" json:"image" validate:"omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string `yaml:"imagePullSecrets,omitempty" json:"imagePullSecrets,omitempty" validate:"omitempty"`

	// Command — container entrypoint command.
	// Each element is resolved independently — template expressions are supported per element.
	// e.g. ["sh", "-c", "echo cleaning up {{ .metadata.name }}"]
	Command []string `yaml:"command,omitempty" json:"command,omitempty" validate:"omitempty"`

	// Args — arguments passed to the container command.
	// Each element supports template expressions independently.
	Args []string `yaml:"args,omitempty" json:"args,omitempty" validate:"omitempty"`

	// BackoffLimit — number of Pod restart attempts before the Job is marked Failed.
	// Default: 3.
	BackoffLimit int `yaml:"backoffLimit,omitempty" json:"backoffLimit,omitempty" validate:"omitempty"`

	// Namespace — target namespace.
	// Default when omitted: "{{ .metadata.namespace }}"
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" validate:"omitempty"`

	// Labels — applied to Job metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty" validate:"omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `yaml:"nodeSelector,omitempty" json:"nodeSelector,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// +optional
	ServiceAccountName string `yaml:"serviceAccountName,omitempty" json:"serviceAccountName,omitempty"`

	// When lists conditions that must all pass for this Job to be created; see
	// Condition. A Job is not removed when they stop passing: it runs to completion.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Reconcile has no effect on a Job entry. Jobs are always a one-time,
	// idempotent create — Inrun never re-applies or updates a Job after it
	// runs, since Jobs are meant to run once to completion. This field exists
	// for schema consistency with other resource types only.
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty" validate:"omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// WorkingDirectory sets the container's working directory (container.WorkingDir).
	// Useful for Git-backed pipelines where build/test commands must run inside
	// a checked-out repository path.
	WorkingDirectory string `yaml:"workingDirectory,omitempty" json:"workingDirectory,omitempty"`

	// Resources — CPU and memory requests/limits for the container.
	// Set resources.preset for a named preset, or resources.requests/limits for
	// explicit values. Preset and explicit values are mutually exclusive.
	//
	//	resources:
	//	  preset: burst
	Resources *ResourceRequirements `yaml:"resources,omitempty" json:"resources,omitempty" validate:"omitempty"`

	// SecurityContext — container-level security settings.
	// Set securityContext.preset for a named preset (baseline, restricted, hardened)
	// or declare individual fields. Preset and explicit fields are mutually exclusive.
	//
	//	securityContext:
	//	  preset: restricted
	SecurityContext *ContainerSecurityContext `yaml:"securityContext,omitempty" json:"securityContext,omitempty"`

	// PodSecurity — pod-level security settings applied to the pod spec.
	// Set podSecurity.preset for a named preset or declare individual fields.
	//
	//	podSecurity:
	//	  preset: baseline
	PodSecurity *PodSecurityContext `yaml:"podSecurity,omitempty" json:"podSecurity,omitempty"`

	// Volumes — pod volumes available for mounting into the container.
	//
	//	volumes:
	//	  - name: config
	//	    configMap:
	//	      name: myapp-config
	Volumes []VolumeSource `yaml:"volumes,omitempty" json:"volumes,omitempty"`

	// VolumeMounts — mounts for the primary container.
	//
	//	volumeMounts:
	//	  - name: config
	//	    mountPath: /etc/myapp
	VolumeMounts []VolumeMount `yaml:"volumeMounts,omitempty" json:"volumeMounts,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// ── CronJob ───────────────────────────────────────────────────────────────────

// CronJobTemplateSource declares one CronJob to be managed by Inrun.
//
// Example:
//
//	onCreate:
//	  cronJobs:
//	    - name: "{{ .metadata.name }}-sync"
//	      schedule: "{{ .spec.syncSchedule }}"
//	      image: "{{ .spec.syncImage }}"
//	      command: ["sh", "-c", "sync.sh"]
type CronJobTemplateSource struct {
	// Name — CronJob name.
	// Default when omitted: "{{ .metadata.name }}-cronjob"
	Name string `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`

	// Schedule — cron schedule expression. Required.
	// Static: "0 * * * *" (every hour)
	// Dynamic: "{{ .spec.schedule }}"
	Schedule string `yaml:"schedule" json:"schedule" validate:"required"`

	// Image — container image. Required.
	Image string `yaml:"image" json:"image" validate:"omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string `yaml:"imagePullSecrets,omitempty" json:"imagePullSecrets,omitempty" validate:"omitempty"`

	// Command — container entrypoint. Each element supports template expressions.
	Command []string `yaml:"command,omitempty" json:"command,omitempty" validate:"omitempty"`

	// Args — container arguments. Each element supports template expressions.
	Args []string `yaml:"args,omitempty" json:"args,omitempty" validate:"omitempty"`

	// Namespace — target namespace.
	// Default when omitted: "{{ .metadata.namespace }}"
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" validate:"omitempty"`

	// Labels — applied to CronJob metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty" validate:"omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `yaml:"nodeSelector,omitempty" json:"nodeSelector,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// +optional
	ServiceAccountName string `yaml:"serviceAccountName,omitempty" json:"serviceAccountName,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty" validate:"omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Suspend — when "true", the schedule stops firing new Jobs; existing runs
	// are unaffected. Accepts a template expression. Default: "false".
	Suspend string `yaml:"suspend,omitempty" json:"suspend,omitempty"`

	// SuccessfulJobsHistoryLimit — number of completed, successful Jobs to
	// keep around for inspection. Default: 3.
	SuccessfulJobsHistoryLimit string `yaml:"successfulJobsHistoryLimit,omitempty" json:"successfulJobsHistoryLimit,omitempty"`

	// FailedJobsHistoryLimit — number of failed Jobs to keep around for
	// inspection. Default: 1.
	FailedJobsHistoryLimit string `yaml:"failedJobsHistoryLimit,omitempty" json:"failedJobsHistoryLimit,omitempty"`

	// ConcurrencyPolicy — how to handle a scheduled run that overlaps with a
	// still-running previous Job.
	// Accepted values: allow (default — runs concurrently), forbid (skip the
	// new run), replace (cancel the running Job and start the new one).
	ConcurrencyPolicy string `yaml:"concurrencyPolicy,omitempty" json:"concurrencyPolicy,omitempty"`

	// StartingDeadlineSeconds — if a scheduled run is missed (e.g. the
	// controller was down) by more than this many seconds, it is counted as
	// failed instead of started late. Omit for no deadline.
	StartingDeadlineSeconds string `yaml:"startingDeadlineSeconds,omitempty" json:"startingDeadlineSeconds,omitempty"`

	// Resources — CPU and memory requests/limits for the container.
	// Set resources.preset for a named preset, or resources.requests/limits for
	// explicit values. Preset and explicit values are mutually exclusive.
	//
	//	resources:
	//	  preset: burst
	Resources *ResourceRequirements `yaml:"resources,omitempty" json:"resources,omitempty" validate:"omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// WorkingDirectory sets the container's working directory (container.WorkingDir).
	// Useful for Git-backed pipelines where build/test commands must run inside
	// a checked-out repository path.
	WorkingDirectory string `yaml:"workingDirectory,omitempty" json:"workingDirectory,omitempty"`

	// SecurityContext — container-level security settings.
	// Set securityContext.preset for a named preset (baseline, restricted, hardened)
	// or declare individual fields. Preset and explicit fields are mutually exclusive.
	//
	//	securityContext:
	//	  preset: restricted
	SecurityContext *ContainerSecurityContext `yaml:"securityContext,omitempty" json:"securityContext,omitempty"`

	// PodSecurity — pod-level security settings applied to the pod spec.
	// Set podSecurity.preset for a named preset or declare individual fields.
	//
	//	podSecurity:
	//	  preset: baseline
	PodSecurity *PodSecurityContext `yaml:"podSecurity,omitempty" json:"podSecurity,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
