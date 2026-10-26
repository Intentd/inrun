// pkg/types/types_pod.go
package types

// ── Pod ───────────────────────────────────────────────────────────────────────

// PodTemplateSource declares one Pod to be managed by Inrun.
//
// Prefer DeploymentTemplateSource for long-running workloads.
// Deployments manage Pod restarts, rolling updates, and replica sets automatically.
// Use PodTemplateSource only when you need direct, single-instance Pod control.
//
// Example:
//
//	onCreate:
//	  pods:
//	    - name: "{{ .metadata.name }}-worker"
//	      image: "{{ .spec.workerImage }}"
//	      port: "9090"
type PodTemplateSource struct {
	// Name — Pod name.
	// Default when omitted: "{{ .metadata.name }}-pod"
	Name string `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`

	// Image — container image. Required.
	// Static: "busybox:1.35" or Dynamic: "{{ .spec.image }}"
	Image string `yaml:"image" json:"image" validate:"omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string `yaml:"imagePullSecrets,omitempty" json:"imagePullSecrets,omitempty" validate:"omitempty"`

	// Port — container port as a string.
	// Static: "8080" or Dynamic: "{{ .spec.port }}"
	Port string `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty"`

	// Protocol — network protocol for the container port.
	// Accepted values: TCP (default), UDP, SCTP. Omit to use TCP.
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty" validate:"omitempty"`

	// Namespace — target namespace.
	// Default when omitted: "{{ .metadata.namespace }}"
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" validate:"omitempty"`

	// Labels — applied to Pod metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty" validate:"omitempty"`

	// Annotations — applied to Pod metadata. Values support template expressions.
	Annotations Labels `yaml:"annotations,omitempty" json:"annotations,omitempty" validate:"omitempty"`

	// Resources — CPU and memory requests/limits for the primary container.
	// Set resources.preset for a named preset, or resources.requests/limits for
	// explicit values. Preset and explicit values are mutually exclusive.
	//
	//	resources:
	//	  preset: burst
	//
	//	resources:
	//	  requests:
	//	    cpu: 100m
	//	    memory: 128Mi
	//	  limits:
	//	    cpu: 500m
	//	    memory: 512Mi
	Resources *ResourceRequirements `yaml:"resources,omitempty" json:"resources,omitempty" validate:"omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `yaml:"nodeSelector,omitempty" json:"nodeSelector,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	ServiceAccountName string `yaml:"serviceAccountName,omitempty" json:"serviceAccountName,omitempty"`

	// RestartPolicy — Always (the Kubernetes default), OnFailure or Never.
	// Supports template expressions.
	RestartPolicy string `yaml:"restartPolicy,omitempty" json:"restartPolicy,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

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

	// Probes — startup, liveness, and readiness probe configuration.
	//
	//	probes:
	//	  liveness:
	//	    type: http
	//	    path: /healthz
	//	    preset: standard
	Probes *ProbesConfig `yaml:"probes,omitempty" json:"probes,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile, not just on create. Equivalent to declaring the same entry under
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty" validate:"omitempty"`

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

	// Containers — the pod's containers, when it runs more than one. With
	// containers set, image, command, args, port, probes and resources move
	// into each container, and this entry's env, envFrom, securityContext and
	// volumeMounts become defaults every container inherits. See Container.
	Containers []Container `yaml:"containers,omitempty" json:"containers,omitempty"`

	// InitContainers — run in order, each to completion, before the containers
	// start. They inherit the same defaults; probes are not allowed. See Container.
	InitContainers []Container `yaml:"initContainers,omitempty" json:"initContainers,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
