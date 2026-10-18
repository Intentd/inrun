// pkg/children/foreach.go
//
// forEach expansion — expands template sources with a forEach declaration
// into N resolved sources, one per element in the list field.
//
// forEach works on every resource type. The expansion happens before the
// resource-specific run_*.go function is called — each run_*.go function
// receives an already-expanded slice and is unaware of forEach.
//
// Expansion sequence in runTemplateReconcile:
//
//	deployments := ExpandForEachDeployments(resolver, t.Deployments)
//	runDeployments(ctx, kube, resolver, obj, deployments, update)
//
// YAML:
//
//	onReconcile:
//	  deployments:
//	    - name: "{{ .metadata.name }}-{{ .item }}"
//	      image: "{{ .spec.image }}"
//	      forEach:
//	        field: spec.regions
//	        as: item
//
// For CR with spec.regions: ["us-east-1", "eu-west-1"]:
// Produces two DeploymentTemplateSources:
//
//	{Name: "my-app-us-east-1", Image: "nginx:latest", ...}
//	{Name: "my-app-eu-west-1", Image: "nginx:latest", ...}
//
// The expansion resolves ALL template expressions immediately — the
// returned slice contains static (non-template) values ready for the
// registry functions.
//
// when: and or: on forEach sources are evaluated per-item — each
// expanded source may pass or fail conditions independently.
package children

import (
	"sort"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ─────────────────────────────────────────────────────────────────────────────
// Generic core
// ─────────────────────────────────────────────────────────────────────────────

// expandForEach is the single generic forEach loop shared by all resource types.
//
// getForEach extracts the *ForEachSpec from one source element.
// resolve receives an item-scoped resolver and a copy of src; it must clear
// ForEach on the copy, resolve all template fields, and return the result.
func expandForEach[T any](
	resolver *template.Resolver,
	srcs []T,
	getForEach func(T) *types.ForEachSpec,
	resolve func(ir *template.Resolver, src T) T,
) []T {
	if !anyHasForEach(len(srcs), func(i int) *types.ForEachSpec { return getForEach(srcs[i]) }) {
		return srcs // fast path — no forEach in this list
	}
	var result []T
	for _, src := range srcs {
		fe := getForEach(src)
		if fe == nil {
			result = append(result, src)
			continue
		}
		for i, fi := range resolveForEachItems(resolver.Data(), fe.Field) {
			ir := itemResolver(resolver, fi, fe.As, i)
			result = append(result, resolve(ir, src))
		}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared field-resolution helpers
// ─────────────────────────────────────────────────────────────────────────────

func resolveEnvVars(ir *template.Resolver, vars types.EnvVarList) types.EnvVarList {
	if len(vars) == 0 {
		return vars
	}
	out := make(types.EnvVarList, 0, len(vars))
	for _, v := range vars {
		rv, _ := ir.Resolve(v.Value)
		out = append(out, types.EnvVar{Name: v.Name, Value: rv})
	}
	return out
}

// resolveMap resolves template expressions in each value of a map[string]string —
// used for labels, annotations, and match selectors alike. Keys are never
// template expressions — only values are resolved.
func resolveMap(ir *template.Resolver, m map[string]string) map[string]string {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		rv, _ := ir.Resolve(v)
		out[k] = rv
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Public ExpandForEach* functions — one per resource type
// ─────────────────────────────────────────────────────────────────────────────

func ExpandForEachNamespaces(
	resolver *template.Resolver,
	srcs []types.NamespaceTemplateSource,
) []types.NamespaceTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.NamespaceTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.NamespaceTemplateSource) types.NamespaceTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Labels = resolveMap(ir, src.Labels)
			if len(src.Finalizers) > 0 {
				out := make([]string, 0, len(src.Finalizers))
				for _, f := range src.Finalizers {
					rv, _ := ir.Resolve(f)
					out = append(out, rv)
				}
				src.Finalizers = out
			}
			return src
		},
	)
}

func ExpandForEachDeployments(
	resolver *template.Resolver,
	srcs []types.DeploymentTemplateSource,
) []types.DeploymentTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.DeploymentTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.DeploymentTemplateSource) types.DeploymentTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Image, _ = ir.Resolve(src.Image)
			src.Replicas, _ = ir.Resolve(src.Replicas)
			src.Port, _ = ir.Resolve(src.Port)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Env = resolveEnvVars(ir, src.Env)
			src.Labels = resolveMap(ir, src.Labels)
			src.Annotations = resolveMap(ir, src.Annotations)
			return src
		},
	)
}

func ExpandForEachReplicaSets(
	resolver *template.Resolver,
	srcs []types.ReplicaSetTemplateSource,
) []types.ReplicaSetTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ReplicaSetTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ReplicaSetTemplateSource) types.ReplicaSetTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Image, _ = ir.Resolve(src.Image)
			src.Replicas, _ = ir.Resolve(src.Replicas)
			src.Port, _ = ir.Resolve(src.Port)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Env = resolveEnvVars(ir, src.Env)
			src.Labels = resolveMap(ir, src.Labels)
			src.Annotations = resolveMap(ir, src.Annotations)
			return src
		},
	)
}

func ExpandForEachServices(
	resolver *template.Resolver,
	srcs []types.ServiceTemplateSource,
) []types.ServiceTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ServiceTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ServiceTemplateSource) types.ServiceTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Port, _ = ir.Resolve(src.Port)
			src.TargetPort, _ = ir.Resolve(src.TargetPort)
			src.Labels = resolveMap(ir, src.Labels)
			src.Selector = resolveMap(ir, src.Selector)
			return src
		},
	)
}

func ExpandForEachSecrets(
	resolver *template.Resolver,
	srcs []types.SecretTemplateSource,
) []types.SecretTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.SecretTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.SecretTemplateSource) types.SecretTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachConfigMaps(
	resolver *template.Resolver,
	srcs []types.ConfigMapTemplateSource,
) []types.ConfigMapTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ConfigMapTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ConfigMapTemplateSource) types.ConfigMapTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachJobs(
	resolver *template.Resolver,
	srcs []types.JobTemplateSource,
) []types.JobTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.JobTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.JobTemplateSource) types.JobTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Image, _ = ir.Resolve(src.Image)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachCronJobs(
	resolver *template.Resolver,
	srcs []types.CronJobTemplateSource,
) []types.CronJobTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.CronJobTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.CronJobTemplateSource) types.CronJobTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Schedule, _ = ir.Resolve(src.Schedule)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachIngresses(
	resolver *template.Resolver,
	srcs []types.IngressTemplateSource,
) []types.IngressTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.IngressTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.IngressTemplateSource) types.IngressTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Host, _ = ir.Resolve(src.Host)
			src.ServiceName, _ = ir.Resolve(src.ServiceName)
			src.ServicePort, _ = ir.Resolve(src.ServicePort)
			src.Path, _ = ir.Resolve(src.Path)
			src.IngressClass, _ = ir.Resolve(src.IngressClass)
			src.Labels = resolveMap(ir, src.Labels)
			src.Annotations = resolveMap(ir, src.Annotations)
			if src.TLS != nil {
				resolved := *src.TLS
				resolved.SecretName, _ = ir.Resolve(src.TLS.SecretName)
				if len(src.TLS.Hosts) > 0 {
					resolved.Hosts = make([]string, 0, len(src.TLS.Hosts))
					for _, h := range src.TLS.Hosts {
						rv, _ := ir.Resolve(h)
						resolved.Hosts = append(resolved.Hosts, rv)
					}
				}
				src.TLS = &resolved
			}
			return src
		},
	)
}

func ExpandForEachHPAs(
	resolver *template.Resolver,
	srcs []types.HPATemplateSource,
) []types.HPATemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.HPATemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.HPATemplateSource) types.HPATemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.ScaleTargetRef.APIVersion, _ = ir.Resolve(src.ScaleTargetRef.APIVersion)
			src.ScaleTargetRef.Kind, _ = ir.Resolve(src.ScaleTargetRef.Kind)
			src.ScaleTargetRef.Name, _ = ir.Resolve(src.ScaleTargetRef.Name)
			src.MinReplicas, _ = ir.Resolve(src.MinReplicas)
			src.MaxReplicas, _ = ir.Resolve(src.MaxReplicas)
			src.TargetCPUUtilizationPercentage, _ = ir.Resolve(src.TargetCPUUtilizationPercentage)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachPDBs(
	resolver *template.Resolver,
	srcs []types.PDBTemplateSource,
) []types.PDBTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.PDBTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.PDBTemplateSource) types.PDBTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.MinAvailable, _ = ir.Resolve(src.MinAvailable)
			src.MaxUnavailable, _ = ir.Resolve(src.MaxUnavailable)
			src.Labels = resolveMap(ir, src.Labels)
			src.Selector = resolveMap(ir, src.Selector)
			return src
		},
	)
}

func ExpandForEachServiceAccounts(
	resolver *template.Resolver,
	srcs []types.ServiceAccountTemplateSource,
) []types.ServiceAccountTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ServiceAccountTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ServiceAccountTemplateSource) types.ServiceAccountTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachStatefulSets(
	resolver *template.Resolver,
	srcs []types.StatefulSetTemplateSource,
) []types.StatefulSetTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.StatefulSetTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.StatefulSetTemplateSource) types.StatefulSetTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Image, _ = ir.Resolve(src.Image)
			src.Tag, _ = ir.Resolve(src.Tag)
			src.Replicas, _ = ir.Resolve(src.Replicas)
			src.Port, _ = ir.Resolve(src.Port)
			src.ServiceName, _ = ir.Resolve(src.ServiceName)
			for i, vct := range src.VolumeClaimTemplates {
				src.VolumeClaimTemplates[i].StorageClass, _ = ir.Resolve(vct.StorageClass)
				src.VolumeClaimTemplates[i].StorageSize, _ = ir.Resolve(vct.StorageSize)
				src.VolumeClaimTemplates[i].MountPath, _ = ir.Resolve(vct.MountPath)
				src.VolumeClaimTemplates[i].Name, _ = ir.Resolve(vct.Name)
			}
			src.Labels = resolveMap(ir, src.Labels)
			src.Annotations = resolveMap(ir, src.Annotations)
			return src
		},
	)
}

func ExpandForEachPVCs(
	resolver *template.Resolver,
	srcs []types.PVCTemplateSource,
) []types.PVCTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.PVCTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.PVCTemplateSource) types.PVCTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.StorageClassName, _ = ir.Resolve(src.StorageClassName)
			src.Storage, _ = ir.Resolve(src.Storage)
			src.VolumeName, _ = ir.Resolve(src.VolumeName)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachPVs(
	resolver *template.Resolver,
	srcs []types.PVTemplateSource,
) []types.PVTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.PVTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.PVTemplateSource) types.PVTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.StorageClassName, _ = ir.Resolve(src.StorageClassName)
			src.Capacity, _ = ir.Resolve(src.Capacity)
			src.ReclaimPolicy, _ = ir.Resolve(src.ReclaimPolicy)
			src.HostPath, _ = ir.Resolve(src.HostPath)
			src.CSIDriver, _ = ir.Resolve(src.CSIDriver)
			src.CSIVolumeHandle, _ = ir.Resolve(src.CSIVolumeHandle)
			src.Labels = resolveMap(ir, src.Labels)
			return src
		},
	)
}

func ExpandForEachRoles(
	resolver *template.Resolver,
	srcs []types.RoleTemplateSource,
) []types.RoleTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.RoleTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.RoleTemplateSource) types.RoleTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			return src
		},
	)
}

func ExpandForEachRoleBindings(
	resolver *template.Resolver,
	srcs []types.RoleBindingTemplateSource,
) []types.RoleBindingTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.RoleBindingTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.RoleBindingTemplateSource) types.RoleBindingTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			return src
		},
	)
}

func ExpandForEachPods(
	resolver *template.Resolver,
	srcs []types.PodTemplateSource,
) []types.PodTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.PodTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.PodTemplateSource) types.PodTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Image, _ = ir.Resolve(src.Image)
			src.Port, _ = ir.Resolve(src.Port)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			src.Labels = resolveMap(ir, src.Labels)
			src.Annotations = resolveMap(ir, src.Annotations)
			return src
		},
	)
}

func ExpandForEachNetworkPolicies(
	resolver *template.Resolver,
	srcs []types.NetworkPolicyTemplateSource,
) []types.NetworkPolicyTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.NetworkPolicyTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.NetworkPolicyTemplateSource) types.NetworkPolicyTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			return src
		},
	)
}

func ExpandForEachResourceQuotas(
	resolver *template.Resolver,
	srcs []types.ResourceQuotaTemplateSource,
) []types.ResourceQuotaTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ResourceQuotaTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ResourceQuotaTemplateSource) types.ResourceQuotaTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			return src
		},
	)
}

func ExpandForEachLimitRanges(
	resolver *template.Resolver,
	srcs []types.LimitRangeTemplateSource,
) []types.LimitRangeTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.LimitRangeTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.LimitRangeTemplateSource) types.LimitRangeTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			src.Namespace, _ = ir.Resolve(src.Namespace)
			return src
		},
	)
}

func ExpandForEachClusterRoles(
	resolver *template.Resolver,
	srcs []types.ClusterRoleTemplateSource,
) []types.ClusterRoleTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ClusterRoleTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ClusterRoleTemplateSource) types.ClusterRoleTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			return src
		},
	)
}

func ExpandForEachClusterRoleBindings(
	resolver *template.Resolver,
	srcs []types.ClusterRoleBindingTemplateSource,
) []types.ClusterRoleBindingTemplateSource {
	return expandForEach(resolver, srcs,
		func(s types.ClusterRoleBindingTemplateSource) *types.ForEachSpec { return s.ForEach },
		func(ir *template.Resolver, src types.ClusterRoleBindingTemplateSource) types.ClusterRoleBindingTemplateSource {
			src.ForEach = nil
			src.Name, _ = ir.Resolve(src.Name)
			return src
		},
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// forEachItem is one iteration step produced by resolveForEachItems.
// For list fields: key = element value, value = nil.
// For map fields:  key = map key (string), value = map value (object or string).
type forEachItem struct {
	key   interface{}
	value interface{}
}

// resolveForEachItems navigates a dot-notation path and returns iteration items.
//
// List field → one item per element; item.value is nil.
//
//	spec.regions: [us-east-1, eu-west-1]
//	→ [{key:"us-east-1"}, {key:"eu-west-1"}]
//
// Map field → one item per key (sorted); item.value is the map value.
//
//	spec.regions: {us-east-1: {replicas: 3}, eu-west-1: {replicas: 1}}
//	→ [{key:"eu-west-1", value:{replicas:1}}, {key:"us-east-1", value:{replicas:3}}]
//
// Template access for map items:
//
//	{{ .item }}            → map key  ("us-east-1")
//	{{ .<as> }}            → same as .item
//	{{ .value.replicas }}  → nested field in the map value
func resolveForEachItems(data map[string]interface{}, path string) []forEachItem {
	var current interface{} = data
	for _, part := range splitFieldPath(path) {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}

	if list, ok := current.([]interface{}); ok {
		items := make([]forEachItem, len(list))
		for i, v := range list {
			items[i] = forEachItem{key: v}
		}
		return items
	}

	if m, ok := current.(map[string]interface{}); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		items := make([]forEachItem, len(keys))
		for i, k := range keys {
			items[i] = forEachItem{key: k, value: m[k]}
		}
		return items
	}

	return nil
}

// itemResolver returns an item-scoped resolver for one forEach iteration step.
// For list items (fi.value == nil) only .item is injected.
// For map items (fi.value != nil) both .item and .value are injected.
func itemResolver(base *template.Resolver, fi forEachItem, as string, index int) *template.Resolver {
	if fi.value != nil {
		return base.WithItemAndValue(fi.key, fi.value, as, index)
	}
	return base.WithItem(fi.key, as, index)
}

// splitFieldPath splits a dot-notation path into keys (see structured.SplitField).
func splitFieldPath(path string) []string {
	return structured.SplitField(path)
}

// anyHasForEach returns true if any element has a non-nil ForEach.
// Used as a fast-path check to avoid allocation when no forEach is declared.
func anyHasForEach(n int, getForEach func(int) *types.ForEachSpec) bool {
	for i := 0; i < n; i++ {
		if getForEach(i) != nil {
			return true
		}
	}
	return false
}
