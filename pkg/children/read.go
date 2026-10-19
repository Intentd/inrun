package children

import (
	"context"
	"fmt"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// readResourceGroup reads one or more resources of the same type and returns
// a map[name → objectMap] for all that were found.
// Missing resources are omitted silently — they may not exist yet on the first reconcile.
func readResourceGroup(
	ctx context.Context,
	kube kubeclient.Interface,
	obj domain.Object,
	resolver *template.Resolver,
	gvr schema.GroupVersionResource,
	names []resolvedChildName,
) map[string]interface{} {
	result := map[string]interface{}{}

	for _, child := range names {
		ns := child.namespace
		if ns == "" {
			ns = obj.GetNamespace()
		}

		var err error
		var u *unstructured.Unstructured
		if ns != "" {
			// Namespaced — use resolved namespace (template or owner fallback)
			u, err = kube.DynamicClient().
				Resource(gvr).
				Namespace(ns).
				Get(ctx, child.name, metav1.GetOptions{})
		} else {
			// Cluster-scoped (e.g. Namespace, ClusterRole)
			u, err = kube.DynamicClient().
				Resource(gvr).
				Get(ctx, child.name, metav1.GetOptions{})
		}

		if err != nil {
			// Not found on first reconcile is expected and not an error.
			// Any other error is worth logging at debug level.
			if !strings.Contains(err.Error(), "not found") {
				logger.FromContext(ctx).Debug().
					Str("resource", fmt.Sprintf("%s/%s", ns, child.name)).
					Str("gvr", gvr.String()).
					Err(err).
					Msg("children: read failed — omitted from status context")
			}
			continue
		}

		// Ensure status is always a non-nil map so template expressions like
		// {{ .children.deployment.status.readyReplicas }} resolve to "" rather
		// than causing a nil-pointer dereference on a newly-created resource.
		o := u.Object
		if s, ok := o["status"]; !ok || s == nil {
			o["status"] = map[string]interface{}{}
		}
		result[child.name] = o
	}

	return result
}

// readCustomResourceGroup reads each custom resource entry using its own GVR resolved
// via the RESTMapper. Unlike built-in types, custom resources may have different
// APIVersions/Kinds within the same list, so GVR must be resolved per entry.
func readCustomResourceGroup(
	ctx context.Context,
	kube kubeclient.Interface,
	obj domain.Object,
	resolver *template.Resolver,
	srcs []types.CustomResourceTemplateSource,
) map[string]interface{} {
	result := map[string]interface{}{}

	expanded := ExpandForEachCustomResources(resolver, srcs)
	for i := range expanded {
		src := &expanded[i]

		name, err := resolver.Resolve(src.Metadata.Name)
		if err != nil || name == "" {
			continue
		}

		gvr, err := src.ResolveGVR(kube.RESTMapper())
		if err != nil {
			logger.FromContext(ctx).Debug().
				Str("resource", name).
				Str("gvk", src.GVKString()).
				Err(err).
				Msg("children: failed to resolve GVR for custom resource — omitted from status context")
			continue
		}

		ns, _ := resolver.Resolve(src.Metadata.Namespace)
		if ns == "" {
			ns = obj.GetNamespace()
		}

		// hasStatus: false → skip this resource entirely (no status to propagate).
		if src.HasStatus != nil && !*src.HasStatus {
			continue
		}

		var u *unstructured.Unstructured
		if src.IsNamespaced() && ns != "" {
			u, err = kube.DynamicClient().Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		} else {
			u, err = kube.DynamicClient().Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		}

		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				logger.FromContext(ctx).Debug().
					Str("resource", fmt.Sprintf("%s/%s", ns, name)).
					Str("gvr", gvr.String()).
					Err(err).
					Msg("children: custom resource read failed — omitted from status context")
			}
			continue
		}

		o := u.Object
		if s, ok := o["status"]; !ok || s == nil {
			o["status"] = map[string]interface{}{}
		}
		result[name] = o
	}

	return result
}

// readEndpointSlicesForServices lists the EndpointSlice for each declared Service
// using the kubernetes.io/service-name label. The result is keyed by service name
// so templates can reference {{ .children.endpointslice }} for single-service blueprints.
func readEndpointSlicesForServices(
	ctx context.Context,
	kube kubeclient.Interface,
	obj domain.Object,
	svcNames []resolvedChildName,
) map[string]interface{} {
	result := map[string]interface{}{}
	for _, svc := range svcNames {
		ns := svc.namespace
		if ns == "" {
			ns = obj.GetNamespace()
		}
		list, err := kube.DynamicClient().
			Resource(EndpointSliceGVR).
			Namespace(ns).
			List(ctx, metav1.ListOptions{
				LabelSelector:   fmt.Sprintf("kubernetes.io/service-name=%s", svc.name),
				Limit:           1,
				ResourceVersion: "0",
			})
		if err != nil || len(list.Items) == 0 {
			continue
		}
		esObj := list.Items[0].Object
		if s, ok := esObj["status"]; !ok || s == nil {
			esObj["status"] = map[string]interface{}{}
		}
		result[svc.name] = esObj
	}
	return result
}

// firstValue returns the first value from a map[string]interface{}.
// Returns a placeholder with an empty status map when the map is empty,
// so templates using {{ .children.deployment.status.readyReplicas }}
// resolve to "" via missingkey=zero rather than a nil-pointer dereference.
// _placeholder:true lets noteExists() distinguish this from a real resource.
func firstValue(m map[string]interface{}) interface{} {
	for _, v := range m {
		return v
	}
	return map[string]interface{}{
		"_placeholder": true,
		"status":       map[string]interface{}{},
	}
}

// mergeTemplates merges onCreate and onReconcile templates into one set.
// We read back resources declared in either block — both produce child resources.
func mergeTemplates(operatorBox *types.OperatorBoxConfig) types.HookTemplates {
	t := types.HookTemplates{}
	if onCreate := operatorBox.EffectiveOnCreate(); onCreate != nil {
		t.Deployments = append(t.Deployments, onCreate.Deployments...)
		t.ReplicaSets = append(t.ReplicaSets, onCreate.ReplicaSets...)
		t.StatefulSets = append(t.StatefulSets, onCreate.StatefulSets...)
		t.Services = append(t.Services, onCreate.Services...)
		t.Secrets = append(t.Secrets, onCreate.Secrets...)
		t.ConfigMaps = append(t.ConfigMaps, onCreate.ConfigMaps...)
		t.Jobs = append(t.Jobs, onCreate.Jobs...)
		t.CronJobs = append(t.CronJobs, onCreate.CronJobs...)
		t.Pods = append(t.Pods, onCreate.Pods...)
		t.ServiceAccounts = append(t.ServiceAccounts, onCreate.ServiceAccounts...)
		t.Namespaces = append(t.Namespaces, onCreate.Namespaces...)
		t.PersistentVolumes = append(t.PersistentVolumes, onCreate.PersistentVolumes...)
		t.PersistentVolumeClaims = append(t.PersistentVolumeClaims, onCreate.PersistentVolumeClaims...)
		t.Ingresses = append(t.Ingresses, onCreate.Ingresses...)
		t.HorizontalPodAutoscalers = append(t.HorizontalPodAutoscalers, onCreate.HorizontalPodAutoscalers...)
		t.StorageClasses = append(t.StorageClasses, onCreate.StorageClasses...)
		t.NetworkPolicies = append(t.NetworkPolicies, onCreate.NetworkPolicies...)
		t.ClusterRoles = append(t.ClusterRoles, onCreate.ClusterRoles...)
		t.ClusterRoleBindings = append(t.ClusterRoleBindings, onCreate.ClusterRoleBindings...)
		t.Roles = append(t.Roles, onCreate.Roles...)
		t.RoleBindings = append(t.RoleBindings, onCreate.RoleBindings...)
		t.LimitRanges = append(t.LimitRanges, onCreate.LimitRanges...)
		t.ResourceQuotas = append(t.ResourceQuotas, onCreate.ResourceQuotas...)
		t.PriorityClasses = append(t.PriorityClasses, onCreate.PriorityClasses...)
		t.CustomResource = append(t.CustomResource, onCreate.CustomResource...)
	}
	if onReconcile := operatorBox.EffectiveOnReconcile(); onReconcile != nil {
		t.Deployments = append(t.Deployments, onReconcile.Deployments...)
		t.ReplicaSets = append(t.ReplicaSets, onReconcile.ReplicaSets...)
		t.StatefulSets = append(t.StatefulSets, onReconcile.StatefulSets...)
		t.Services = append(t.Services, onReconcile.Services...)
		t.Secrets = append(t.Secrets, onReconcile.Secrets...)
		t.ConfigMaps = append(t.ConfigMaps, onReconcile.ConfigMaps...)
		t.Jobs = append(t.Jobs, onReconcile.Jobs...)
		t.CronJobs = append(t.CronJobs, onReconcile.CronJobs...)
		t.Pods = append(t.Pods, onReconcile.Pods...)
		t.ServiceAccounts = append(t.ServiceAccounts, onReconcile.ServiceAccounts...)
		t.Namespaces = append(t.Namespaces, onReconcile.Namespaces...)
		t.PersistentVolumes = append(t.PersistentVolumes, onReconcile.PersistentVolumes...)
		t.PersistentVolumeClaims = append(t.PersistentVolumeClaims, onReconcile.PersistentVolumeClaims...)
		t.Ingresses = append(t.Ingresses, onReconcile.Ingresses...)
		t.HorizontalPodAutoscalers = append(t.HorizontalPodAutoscalers, onReconcile.HorizontalPodAutoscalers...)
		t.StorageClasses = append(t.StorageClasses, onReconcile.StorageClasses...)
		t.NetworkPolicies = append(t.NetworkPolicies, onReconcile.NetworkPolicies...)
		t.ClusterRoles = append(t.ClusterRoles, onReconcile.ClusterRoles...)
		t.ClusterRoleBindings = append(t.ClusterRoleBindings, onReconcile.ClusterRoleBindings...)
		t.Roles = append(t.Roles, onReconcile.Roles...)
		t.RoleBindings = append(t.RoleBindings, onReconcile.RoleBindings...)
		t.LimitRanges = append(t.LimitRanges, onReconcile.LimitRanges...)
		t.ResourceQuotas = append(t.ResourceQuotas, onReconcile.ResourceQuotas...)
		t.PriorityClasses = append(t.PriorityClasses, onReconcile.PriorityClasses...)
		t.CustomResource = append(t.CustomResource, onReconcile.CustomResource...)
	}
	return t
}
