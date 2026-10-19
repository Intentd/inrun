package types

// VisitResources calls fn for every resource template in this HookTemplates.
// It abstracts over all resource slices (Deployments, Services, Jobs, etc.)
// so callers can perform generic operations like validating fields or
// scanning for managed-resource contracts.
func (h *HookTemplates) VisitResources(fn func(res interface{})) {
	// Core workload resources
	for _, x := range h.Deployments {
		fn(x)
	}
	for _, x := range h.ReplicaSets {
		fn(x)
	}
	for _, x := range h.StatefulSets {
		fn(x)
	}
	for _, x := range h.DaemonSets {
		fn(x)
	}
	for _, x := range h.Pods {
		fn(x)
	}

	// Services & networking
	for _, x := range h.Services {
		fn(x)
	}
	for _, x := range h.Ingresses {
		fn(x)
	}
	for _, x := range h.NetworkPolicies {
		fn(x)
	}

	// Batch
	for _, x := range h.Jobs {
		fn(x)
	}
	for _, x := range h.CronJobs {
		fn(x)
	}

	// Config & identity
	for _, x := range h.Secrets {
		fn(x)
	}
	for _, x := range h.ConfigMaps {
		fn(x)
	}
	for _, x := range h.ServiceAccounts {
		fn(x)
	}
	for _, x := range h.Roles {
		fn(x)
	}
	for _, x := range h.RoleBindings {
		fn(x)
	}
	for _, x := range h.ClusterRoles {
		fn(x)
	}
	for _, x := range h.ClusterRoleBindings {
		fn(x)
	}

	// Storage
	for _, x := range h.PersistentVolumes {
		fn(x)
	}
	for _, x := range h.PersistentVolumeClaims {
		fn(x)
	}
	for _, x := range h.StorageClasses {
		fn(x)
	}
	for _, x := range h.StorageLocations {
		fn(x)
	}
	for _, x := range h.StoragePools {
		fn(x)
	}
	for _, x := range h.StorageBackups {
		fn(x)
	}
	for _, x := range h.StorageSnapshots {
		fn(x)
	}
	for _, x := range h.StorageVolumes {
		fn(x)
	}

	// Autoscaling, disruption, scheduling
	for _, x := range h.HorizontalPodAutoscalers {
		fn(x)
	}
	for _, x := range h.PodDisruptionBudgets {
		fn(x)
	}
	for _, x := range h.PriorityClasses {
		fn(x)
	}
	for _, x := range h.RuntimeClasses {
		fn(x)
	}
	for _, x := range h.LimitRanges {
		fn(x)
	}
	for _, x := range h.ResourceQuotas {
		fn(x)
	}

	// Namespaces
	for _, x := range h.Namespaces {
		fn(x)
	}

	// Pod templates
	for _, x := range h.PodTemplates {
		fn(x)
	}

	// Placeholders (future extensibility)
	for _, x := range h.Volumes {
		fn(x)
	}
	for _, x := range h.VolumeMounts {
		fn(x)
	}
	for _, x := range h.ServiceMonitors {
		fn(x)
	}
	for _, x := range h.PodSecurityPolicies {
		fn(x)
	}
	for _, x := range h.PriorityLevelConfigurations {
		fn(x)
	}
	for _, x := range h.CustomResource {
		fn(x)
	}
}
