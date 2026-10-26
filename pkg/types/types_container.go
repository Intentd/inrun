package types

// Container is one container of a pod, for the containers and initContainers
// of every pod kind (pods, deployments, replicaSets, statefulSets, jobs,
// cronJobs). It mirrors kubewright's Container, with port, protocol and
// imagePullPolicy as strings so they take template expressions.
//
// Without containers, an entry's flat fields (image, port, env, ...) describe
// its one container. With containers set, the entry's env, envFrom,
// securityContext and volumeMounts become defaults every container and init
// container inherits (a container's own value wins: env by name, volumeMounts
// by mount path), and the entry's image, command, args, port, probes and
// resources must be empty: they belong to one container.
//
//	deployments:
//	  - name: "{{ .metadata.name }}"
//	    initContainers:
//	      - name: migrate
//	        image: "{{ .spec.image }}"
//	        args: ["migrate", "up"]
//	    containers:
//	      - name: app
//	        image: "{{ .spec.image }}"
//	        port: "8080"
//	        probes:
//	          readiness:
//	            type: http
//	            path: /ready
//	      - name: proxy
//	        image: envoyproxy/envoy:v1.31
//	        port: "9901"
type Container struct {
	// Name — must be unique in the pod. Required. Supports template expressions.
	Name string `yaml:"name" json:"name"`

	// Image — container image. Required. Supports template expressions.
	Image string `yaml:"image" json:"image"`

	// ImagePullPolicy — Always, IfNotPresent or Never. Omit for the Kubernetes default.
	ImagePullPolicy string `yaml:"imagePullPolicy,omitempty" json:"imagePullPolicy,omitempty"`

	// Command — the entrypoint; each element supports template expressions.
	Command []string `yaml:"command,omitempty" json:"command,omitempty"`

	// Args — arguments to the entrypoint; each element supports template expressions.
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// Port — the port the container listens on, as a string. Probes without a
	// port use it. Static: "8080". Dynamic: "{{ .spec.port }}".
	Port string `yaml:"port,omitempty" json:"port,omitempty"`

	// Protocol — for port: TCP (default), UDP or SCTP.
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty"`

	// Env — environment variables, in Kubernetes list format; values support
	// template expressions. Merged over the entry's env by name.
	Env []EnvVar `yaml:"env,omitempty" json:"env,omitempty"`

	// EnvFrom — load every key of Secrets and ConfigMaps as environment variables.
	EnvFrom *EnvFrom `yaml:"envFrom,omitempty" json:"envFrom,omitempty"`

	// Resources — requests and limits, or resources.preset for a named preset.
	Resources *ResourceRequirements `yaml:"resources,omitempty" json:"resources,omitempty"`

	// Probes — startup, liveness and readiness probes. Not allowed on an init container.
	Probes *ProbesConfig `yaml:"probes,omitempty" json:"probes,omitempty"`

	// SecurityContext — container security settings, or securityContext.preset.
	SecurityContext *ContainerSecurityContext `yaml:"securityContext,omitempty" json:"securityContext,omitempty"`

	// VolumeMounts — volumes from the entry's volumes, mounted into this container.
	// Merged over the entry's volumeMounts by mount path.
	VolumeMounts []VolumeMount `yaml:"volumeMounts,omitempty" json:"volumeMounts,omitempty"`
}
