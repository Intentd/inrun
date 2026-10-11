// pkg/runners/surface_sweep.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SweepOwnedNamespacedResources deletes all namespace-scoped resources in ns
// whose inrun-owner label matches ownerKey. Used for surface-switch cleanup
// where template-based deletion cannot work: when a spec field that drove a
// forEach declaration is removed before cleanup runs (e.g. spec.regions is
// cleared when switching away from a regional target), ExpandForEach produces
// nothing and the orphaned resources are never found by name.
//
// This is a label-selector sweep — it finds resources by ownership label
// rather than by resolving template names, so it is immune to spec changes.
func SweepOwnedNamespacedResources(
	ctx context.Context,
	kube kubeclient.Interface,
	ownerKey string,
	ns string,
) error {
	cs := kube.Clientset()
	sel := labels.InrunOwner + "=" + ownerKey
	opts := metav1.ListOptions{LabelSelector: sel}
	dopts := metav1.DeleteOptions{}

	var errs []error
	collect := func(label string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	// ── Workloads ─────────────────────────────────────────────────────────────
	if list, err := cs.AppsV1().Deployments(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("deployment/"+r.Name, cs.AppsV1().Deployments(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.AppsV1().ReplicaSets(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("replicaset/"+r.Name, cs.AppsV1().ReplicaSets(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.AppsV1().StatefulSets(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("statefulset/"+r.Name, cs.AppsV1().StatefulSets(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── Networking ────────────────────────────────────────────────────────────
	if list, err := cs.CoreV1().Services(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("service/"+r.Name, cs.CoreV1().Services(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.NetworkingV1().Ingresses(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("ingress/"+r.Name, cs.NetworkingV1().Ingresses(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.NetworkingV1().NetworkPolicies(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("networkpolicy/"+r.Name, cs.NetworkingV1().NetworkPolicies(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── Config ────────────────────────────────────────────────────────────────
	if list, err := cs.CoreV1().ConfigMaps(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("configmap/"+r.Name, cs.CoreV1().ConfigMaps(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.CoreV1().Secrets(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("secret/"+r.Name, cs.CoreV1().Secrets(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── Storage ───────────────────────────────────────────────────────────────
	if list, err := cs.CoreV1().PersistentVolumeClaims(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("pvc/"+r.Name, cs.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── Autoscaling / policy ──────────────────────────────────────────────────
	if list, err := cs.AutoscalingV2().HorizontalPodAutoscalers(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("hpa/"+r.Name, cs.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.PolicyV1().PodDisruptionBudgets(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("pdb/"+r.Name, cs.PolicyV1().PodDisruptionBudgets(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── RBAC ─────────────────────────────────────────────────────────────────
	if list, err := cs.CoreV1().ServiceAccounts(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("serviceaccount/"+r.Name, cs.CoreV1().ServiceAccounts(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.RbacV1().Roles(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("role/"+r.Name, cs.RbacV1().Roles(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.RbacV1().RoleBindings(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("rolebinding/"+r.Name, cs.RbacV1().RoleBindings(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── Quota / limits ────────────────────────────────────────────────────────
	if list, err := cs.CoreV1().ResourceQuotas(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("resourcequota/"+r.Name, cs.CoreV1().ResourceQuotas(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.CoreV1().LimitRanges(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("limitrange/"+r.Name, cs.CoreV1().LimitRanges(ns).Delete(ctx, r.Name, dopts))
		}
	}

	// ── Pods / CronJobs ───────────────────────────────────────────────────────
	if list, err := cs.CoreV1().Pods(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("pod/"+r.Name, cs.CoreV1().Pods(ns).Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.BatchV1().CronJobs(ns).List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("cronjob/"+r.Name, cs.BatchV1().CronJobs(ns).Delete(ctx, r.Name, dopts))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("surface sweep %q: %v", ownerKey, errs)
	}
	return nil
}

// SweepOwnedClusterScopedResources deletes all cluster-scoped resources whose
// inrun-owner label matches ownerKey. Mirrors SweepOwnedNamespacedResources
// but operates on cluster-scoped types that Kubernetes GC does not cascade.
func SweepOwnedClusterScopedResources(
	ctx context.Context,
	kube kubeclient.Interface,
	ownerKey string,
) error {
	cs := kube.Clientset()
	sel := labels.InrunOwner + "=" + ownerKey
	opts := metav1.ListOptions{LabelSelector: sel}
	dopts := metav1.DeleteOptions{}

	var errs []error
	collect := func(label string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	if list, err := cs.CoreV1().Namespaces().List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("namespace/"+r.Name, cs.CoreV1().Namespaces().Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.RbacV1().ClusterRoles().List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("clusterrole/"+r.Name, cs.RbacV1().ClusterRoles().Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.RbacV1().ClusterRoleBindings().List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("clusterrolebinding/"+r.Name, cs.RbacV1().ClusterRoleBindings().Delete(ctx, r.Name, dopts))
		}
	}
	if list, err := cs.CoreV1().PersistentVolumes().List(ctx, opts); err == nil {
		for _, r := range list.Items {
			collect("pv/"+r.Name, cs.CoreV1().PersistentVolumes().Delete(ctx, r.Name, dopts))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cluster-scoped surface sweep %q: %v", ownerKey, errs)
	}
	return nil
}
