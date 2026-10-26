package runners

// Workload resources: Pods, Deployments, ReplicaSets, StatefulSets and CronJobs.
// Jobs, which wait to complete, are in jobs.go.

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/core/cronjobs"
	"github.com/intentd/kubewright/core/deployments"
	"github.com/intentd/kubewright/core/pods"
	"github.com/intentd/kubewright/core/replicasets"
	"github.com/intentd/kubewright/core/statefulsets"
)

// RunPods writes Pod template entries; runChildren describes how.
func RunPods(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PodTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, podChild)
}

var podChild = child[types.PodTemplateSource, pods.Spec]{
	log:      "pod",
	resource: "Pod",
	field:    "pods",
	entry: func(s types.PodTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolvePod,
	spec:          resources.PodSpec,
	create:        pods.Create,
	apply:         pods.Apply,
	deleteIfOwned: pods.DeleteIfOwned,
}

// RunDeployments writes Deployment template entries; runChildren describes how.
func RunDeployments(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.DeploymentTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, deploymentChild)
}

var deploymentChild = child[types.DeploymentTemplateSource, deployments.Spec]{
	log:      "deployment",
	resource: "Deployment",
	field:    "deployments",
	entry: func(s types.DeploymentTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveDeployment,
	spec:          resources.DeploymentSpec,
	create:        deployments.Create,
	apply:         deployments.Apply,
	deleteIfOwned: deployments.DeleteIfOwned,
	after: func(ctx context.Context, kube kubeclient.Interface, resolver *template.Resolver, owner domain.Object, src types.DeploymentTemplateSource, name, ns string) error {
		if src.Autoscale == nil {
			return nil
		}
		if err := EvaluateWorkloadAutoscaleDeployment(ctx, kube, resolver, owner.GetName(), ns, name, src.Autoscale); err != nil {
			return fmt.Errorf("autoscale: %w", err)
		}
		return nil
	},
}

// RunReplicaSets writes ReplicaSet template entries; runChildren describes how.
func RunReplicaSets(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ReplicaSetTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, replicasetChild)
}

var replicasetChild = child[types.ReplicaSetTemplateSource, replicasets.Spec]{
	log:      "replicaset",
	resource: "ReplicaSet",
	field:    "replicaSets",
	entry: func(s types.ReplicaSetTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveReplicaSet,
	spec:          resources.ReplicaSetSpec,
	create:        replicasets.Create,
	apply:         replicasets.Apply,
	deleteIfOwned: replicasets.DeleteIfOwned,
	after: func(ctx context.Context, kube kubeclient.Interface, resolver *template.Resolver, owner domain.Object, src types.ReplicaSetTemplateSource, name, ns string) error {
		if src.Autoscale == nil {
			return nil
		}
		if err := EvaluateWorkloadAutoscaleReplicaSet(ctx, kube, resolver, owner.GetName(), ns, name, src.Autoscale); err != nil {
			return fmt.Errorf("autoscale: %w", err)
		}
		return nil
	},
}

// RunStatefulSets writes StatefulSet template entries; runChildren describes how.
func RunStatefulSets(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.StatefulSetTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, statefulsetChild)
}

var statefulsetChild = child[types.StatefulSetTemplateSource, statefulsets.Spec]{
	log:      "statefulset",
	resource: "StatefulSet",
	field:    "statefulSets",
	entry: func(s types.StatefulSetTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveStatefulSet,
	spec:          resources.StatefulSetSpec,
	create:        statefulsets.Create,
	apply:         statefulsets.Apply,
	deleteIfOwned: statefulsets.DeleteIfOwned,
	after: func(ctx context.Context, kube kubeclient.Interface, resolver *template.Resolver, owner domain.Object, src types.StatefulSetTemplateSource, name, ns string) error {
		if src.Autoscale == nil {
			return nil
		}
		if err := EvaluateWorkloadAutoscaleStatefulSet(ctx, kube, resolver, owner.GetName(), ns, name, src.Autoscale); err != nil {
			return fmt.Errorf("autoscale: %w", err)
		}
		return nil
	},
}

// RunCronJobs writes CronJob template entries; runChildren describes how.
func RunCronJobs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.CronJobTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, cronjobChild)
}

var cronjobChild = child[types.CronJobTemplateSource, cronjobs.Spec]{
	log:      "cronjob",
	resource: "CronJob",
	field:    "cronJobs",
	entry: func(s types.CronJobTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveCronJob,
	spec:          resources.CronJobSpec,
	create:        cronjobs.Create,
	apply:         cronjobs.Apply,
	deleteIfOwned: cronjobs.DeleteIfOwned,
}
