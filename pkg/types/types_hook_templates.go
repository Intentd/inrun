// pkg/types/types_hook_templates.go
package types

// HookTemplates declares the child resources Inrun manages at a lifecycle event.
// Resources receive owner references — Kubernetes GC handles deletion automatically.
// All slices are optional; undeclared resources are invisible to the reconciler.
type HookTemplates struct {
	// Deployments declared by this block, one entry per object.
	Deployments []DeploymentTemplateSource `yaml:"deployments,omitempty" json:"deployments,omitempty" validate:"omitempty"`
	// ReplicaSets declared by this block.
	ReplicaSets []ReplicaSetTemplateSource `yaml:"replicaSets,omitempty" json:"replicaSets,omitempty" validate:"omitempty"`
	// Services declared by this block.
	Services []ServiceTemplateSource `yaml:"services,omitempty" json:"services,omitempty" validate:"omitempty"`
	// Pods declared by this block.
	Pods []PodTemplateSource `yaml:"pods,omitempty" json:"pods,omitempty" validate:"omitempty"`
	// Jobs declared by this block. In onDelete, Inrun waits for each Job to
	// finish, up to onDelete's timeout (default 5m), before releasing the CR.
	Jobs []JobTemplateSource `yaml:"jobs,omitempty" json:"jobs,omitempty" validate:"omitempty"`
	// CronJobs declared by this block.
	CronJobs []CronJobTemplateSource `yaml:"cronJobs,omitempty" json:"cronJobs,omitempty" validate:"omitempty"`
	// Secrets declared by this block: created, generated, or copied from another namespace.
	Secrets []SecretTemplateSource `yaml:"secrets,omitempty" json:"secrets,omitempty" validate:"omitempty"`
	// ConfigMaps declared by this block, or copied from another namespace.
	ConfigMaps []ConfigMapTemplateSource `yaml:"configMaps,omitempty" json:"configMaps,omitempty" validate:"omitempty"`
	// ServiceAccounts declared by this block.
	ServiceAccounts []ServiceAccountTemplateSource `yaml:"serviceAccounts,omitempty" json:"serviceAccounts,omitempty" validate:"omitempty"`
	// StatefulSets declared by this block.
	StatefulSets []StatefulSetTemplateSource `yaml:"statefulSets,omitempty" json:"statefulSets,omitempty" validate:"omitempty"`
	// Ingresses declared by this block.
	Ingresses []IngressTemplateSource `yaml:"ingresses,omitempty" json:"ingresses,omitempty" validate:"omitempty"`
	// PersistentVolumes declared by this block. Cluster-scoped.
	PersistentVolumes []PVTemplateSource `yaml:"persistentVolumes,omitempty" json:"persistentVolumes,omitempty" validate:"omitempty"`
	// PersistentVolumeClaims declared by this block.
	PersistentVolumeClaims []PVCTemplateSource `yaml:"persistentVolumeClaims,omitempty" json:"persistentVolumeClaims,omitempty" validate:"omitempty"`
	// HorizontalPodAutoscalers declared by this block.
	HorizontalPodAutoscalers []HPATemplateSource `yaml:"hpa,omitempty" json:"hpa,omitempty" validate:"omitempty"`
	// PodDisruptionBudgets declared by this block.
	PodDisruptionBudgets []PDBTemplateSource `yaml:"pdb,omitempty" json:"pdb,omitempty" validate:"omitempty"`
	// Namespaces declared by this block. Cluster-scoped.
	Namespaces []NamespaceTemplateSource `yaml:"namespaces,omitempty" json:"namespaces,omitempty" validate:"omitempty"`
	// Roles declared by this block.
	Roles []RoleTemplateSource `yaml:"roles,omitempty" json:"roles,omitempty" validate:"omitempty"`
	// RoleBindings declared by this block.
	RoleBindings []RoleBindingTemplateSource `yaml:"roleBindings,omitempty" json:"roleBindings,omitempty" validate:"omitempty"`
	// Custom resources of any kind, another Inrun CRD's included, declared by this block.
	CustomResource []CustomResourceTemplateSource `yaml:"custom,omitempty" json:"custom,omitempty" validate:"omitempty"`
	// ClusterRoles declared by this block. Cluster-scoped.
	ClusterRoles []ClusterRoleTemplateSource `yaml:"clusterRoles,omitempty" json:"clusterRoles,omitempty" validate:"omitempty"`
	// ClusterRoleBindings declared by this block. Cluster-scoped.
	ClusterRoleBindings []ClusterRoleBindingTemplateSource `yaml:"clusterRoleBindings,omitempty" json:"clusterRoleBindings,omitempty" validate:"omitempty"`
	// LimitRanges declared by this block, or copied into other namespaces.
	LimitRanges []LimitRangeTemplateSource `yaml:"limitRanges,omitempty" json:"limitRanges,omitempty" validate:"omitempty"`
	// ResourceQuotas declared by this block, or copied into other namespaces.
	ResourceQuotas []ResourceQuotaTemplateSource `yaml:"resourceQuotas,omitempty" json:"resourceQuotas,omitempty" validate:"omitempty"`
	// NetworkPolicies declared by this block, or copied into other namespaces.
	NetworkPolicies []NetworkPolicyTemplateSource `yaml:"networkPolicies,omitempty" json:"networkPolicies,omitempty" validate:"omitempty"`

	// External declares HTTP calls to make before resource creation.
	// Results available as .external.<n>.status, .body, .error
	External []ExternalCallSpec `yaml:"external,omitempty" json:"external,omitempty"`

	// When declares conditions that must all be true for this block to execute.
	// AND semantics — all conditions must pass. Evaluated before any resource in the block runs.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or declares conditions where at least one must be true for this block to execute.
	// Works alongside When: block runs if When passes OR any Or condition passes.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// Ordered controls whether deletion happens sequentially with verification.
	// true  — delete groups in order, verify each is gone before proceeding
	// false — the default: children go with the CR all at once, by owner
	//         reference, or by their owner-uid label in other namespaces
	Ordered bool `yaml:"ordered,omitempty" json:"ordered,omitempty"`

	// Name is an optional identifier for this block, used in logs and events.
	// When set on groups inside ordered deletion, names must be unique across the group list.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Groups declares sequential deletion stages for ordered deletes.
	// Each element is a full HookTemplates block whose resources are deleted
	// as a unit. Inrun deletes stage N, waits until all resources are gone,
	// then deletes stage N+1. Omit when Ordered is false.
	// When Ordered is true and Groups is empty, the flat resource fields above
	// (Jobs, Deployments, …) are treated as a single implicit group.
	Groups []HookTemplates `yaml:"groups,omitempty" json:"groups,omitempty"`

	// Timeout is the maximum time to wait for each deletion group to complete.
	// Defaults to 5m when Ordered is true. Ignored when Ordered is false.
	Timeout *Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}
