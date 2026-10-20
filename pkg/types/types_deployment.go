// pkg/types/types_deployment.go
package types

// ── Deployment ────────────────────────────────────────────────────────────────

// DeploymentTemplateSource declares one Deployment to be managed by Inrun.
//
// Minimal example — static values only:
//
//	onCreate:
//	  deployments:
//	    - image: nginx:1.25
//	      replicas: "3"
//	      port: "8080"
//	      resources:
//	        preset: burst      # Use inrun's burst resource configuration
//
// Full example — dynamic values from the CR:
//
//	onCreate:
//	  deployments:
//	    - name: "{{ .metadata.name }}-app"
//	      image: "{{ .spec.image }}"
//	      replicas: "{{ .spec.replicas }}"
//	      port: "{{ .spec.port }}"
//	      namespace: "{{ .metadata.namespace }}"
//	      labels:
//	        app: "{{ .metadata.name }}"
//	        managed-by: inrun
//	      resources:
//	        requests:
//	          cpu: 100m
//	          memory: 128Mi
//	        limits:
//	          cpu: 500m
//	          memory: 512Mi
type DeploymentTemplateSource struct {
	// Name — Deployment and primary container name.
	// Supports template expressions.
	// Default when omitted: "{{ .metadata.name }}-deployment"
	Name string `yaml:"name,omitempty" json:"name,omitempty" validate:"omitempty"`

	// Image — container image. Required (must be declared here or resolvable
	// from CR). Static: "nginx:1.25". Dynamic: "{{ .spec.image }}".
	Image string `yaml:"image" json:"image" validate:"omitempty"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use
	// for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	ImagePullSecrets []string `yaml:"imagePullSecrets,omitempty" json:"imagePullSecrets,omitempty" validate:"omitempty"`

	// Replicas — number of pod replicas as a string. Static: "3". Dynamic:
	// "{{ .spec.replicas }}". Default: "1".
	Replicas string `yaml:"replicas,omitempty" json:"replicas,omitempty" validate:"omitempty"`

	// Port — primary container port as a string. Static: "8080". Dynamic:
	// "{{ .spec.port }}". Omit to expose no port.
	Port string `yaml:"port,omitempty" json:"port,omitempty" validate:"omitempty"`

	// Protocol — network protocol for the container port.
	// Accepted values: TCP (default), UDP, SCTP.
	// Omit to use TCP.
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty" validate:"omitempty"`

	// Namespace — target namespace for the Deployment.
	// Default when omitted: "{{ .metadata.namespace }}" (same namespace as the CR).
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty" validate:"omitempty"`

	// Labels — applied to the Deployment ObjectMeta and the pod template.
	// Label values support template expressions.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty" validate:"omitempty"`

	// Annotations — applied to the Deployment ObjectMeta only.
	// Annotation values support template expressions.
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

	// Env — environment variables for the primary container, in Kubernetes-native list format.
	// Each entry has a name and either a value or a valueFrom source.
	// If omitted, no environment variables are added.
	//
	//	env:
	//	  - name: LOG_LEVEL
	//	    value: info
	//	  - name: API_KEY
	//	    valueFrom:
	//	      secretKeyRef:
	//	        name: myapp-secrets
	//	        key: api-key
	Env EnvVarList `yaml:"env,omitempty" json:"env,omitempty"`

	// EnvFrom — bulk-load environment variables from Secrets and/or ConfigMaps
	// into the primary container, in addition to any individual entries in env.
	// Each secretRef/configMapRef entry names an existing Secret or ConfigMap;
	// every key in it becomes an environment variable.
	//
	//	envFrom:
	//	  secretRef:
	//	    - name: myapp-secrets
	//	  configMapRef:
	//	    - name: myapp-config
	EnvFrom *EnvFrom `yaml:"envFrom,omitempty" json:"envFrom,omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `yaml:"nodeSelector,omitempty" json:"nodeSelector,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	ServiceAccountName string `yaml:"serviceAccountName,omitempty" json:"serviceAccountName,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty" validate:"omitempty"`

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
	//	# then, elsewhere in this same declaration:
	//	#   name: "{{ .metadata.name }}-{{ .region }}"
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// WorkingDirectory sets the container's working directory (container.WorkingDir).
	// Useful for Git-backed pipelines where build/test commands must run inside
	// a checked-out repository path.
	WorkingDirectory string `yaml:"workingDirectory,omitempty" json:"workingDirectory,omitempty"`

	// Probes — startup, liveness, and readiness probe configuration.
	//
	//	probes:
	//	  liveness:
	//	    type: http
	//	    path: /healthz
	//	    preset: standard
	//	  readiness:
	//	    type: tcp
	//	    preset: fast
	Probes *ProbesConfig `yaml:"probes,omitempty" json:"probes,omitempty"`

	// SecurityContext — container-level security settings.
	// Set securityContext.preset for a named preset (baseline, restricted, hardened)
	// or declare individual fields. Preset and explicit fields are mutually exclusive.
	//
	//	securityContext:
	//	  preset: restricted
	//
	//	securityContext:
	//	  runAsNonRoot: true
	//	  readOnlyRootFilesystem: true
	SecurityContext *ContainerSecurityContext `yaml:"securityContext,omitempty" json:"securityContext,omitempty"`

	// PodSecurity — pod-level security settings applied to the pod spec.
	// Set podSecurity.preset for a named preset or declare individual fields.
	//
	//	podSecurity:
	//	  preset: baseline
	//
	//	podSecurity:
	//	  runAsNonRoot: true
	//	  fsGroup: 1000
	PodSecurity *PodSecurityContext `yaml:"podSecurity,omitempty" json:"podSecurity,omitempty"`

	// RollingUpdate — rolling update strategy for this Deployment.
	// Set rollingUpdate.preset for a named preset (safe, fast, blue-green),
	// or declare maxSurge/maxUnavailable explicitly.
	// Preset and explicit fields are mutually exclusive.
	//
	//	rollingUpdate:
	//	  preset: safe
	//
	//	rollingUpdate:
	//	  maxSurge: "1"
	//	  maxUnavailable: "0"
	RollingUpdate *RollingUpdateBehavior `yaml:"rollingUpdate,omitempty" json:"rollingUpdate,omitempty"`

	// Command overrides the container entrypoint command. Each element is a command token.
	// Supports template expressions.
	//
	//	command: ["sh", "-c", "echo {{ .spec.greeting }}"]
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`

	// Volumes — pod volumes available for mounting into the container.
	// Supports configMap, secret, emptyDir, persistentVolumeClaim, and hostPath sources.
	// Volume names support template expressions.
	//
	//	volumes:
	//	  - name: config
	//	    configMap:
	//	      name: myapp-config
	//	  - name: cache
	//	    emptyDir: {}
	Volumes []VolumeSource `yaml:"volumes,omitempty" json:"volumes,omitempty"`

	// VolumeMounts — mounts for the primary container. Each entry references a
	// volume declared in volumes: by name.
	//
	//	volumeMounts:
	//	  - name: config
	//	    mountPath: /etc/myapp
	//	  - name: cache
	//	    mountPath: /var/cache/myapp
	VolumeMounts []VolumeMount `yaml:"volumeMounts,omitempty" json:"volumeMounts,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`

	// Autoscale declares workload autoscaling behaviour for this Deployment.
	// When set, the reconciler evaluates scale-up and scale-down conditions on every
	// reconcile and patches spec.replicas when conditions pass and cooldown has elapsed.
	// No goroutine — the resync period is the evaluation tick.
	//
	//	autoscale:
	//	  min: 2
	//	  max: 10
	//	  cooldown: 2m
	//	  scaleUp:
	//	    conditions:
	//	      when:
	//	        - field: "{{ promAboveThreshold .external.cpu.raw 80 }}"
	//	          equals: "true"
	//	    increment: 1
	//	  scaleDown:
	//	    conditions:
	//	      when:
	//	        - field: "{{ promBelowThreshold .external.cpu.raw 20 }}"
	//	          equals: "true"
	//	    decrement: 1
	Autoscale *WorkloadAutoscale `yaml:"autoscale,omitempty" json:"autoscale,omitempty"`
}
