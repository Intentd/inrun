// pkg/reconciler/run_delete_ordered.go
//
// Sequential deletion with completion gates.
//
// When onDelete.ordered: true the reconciler deletes resources in stages
// rather than relying on garbage collection. Each stage is a HookTemplates
// block. After submitting all deletes in a stage the reconciler polls the
// API server until every resource is confirmed gone, then advances to the
// next stage.
//
// Deletion order within a stage is undefined; order is enforced between stages.
package generic

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/runners"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const defaultOrderedDeleteTimeout = 5 * time.Minute
const orderedDeletePollInterval = 3 * time.Second

// orderedDeleteEntry identifies a single resource to wait on after deletion.
type orderedDeleteEntry struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
}

// runOrderedDelete deletes resource groups sequentially.
// For each group: submit all deletes, then poll until every resource is gone.
func (r *Reconciler[PTR]) runOrderedDelete(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	t *types.HookTemplates,
	guard func(ctx context.Context, obj domain.Object, ns string) bool,
) error {
	log := logger.FromContext(ctx)
	log.Info().Str("name", obj.GetName()).Msg("ordered delete: starting sequential cleanup")

	timeout := defaultOrderedDeleteTimeout
	if t.Timeout != nil && t.Timeout.Duration > 0 {
		timeout = t.Timeout.Duration
	}

	// Build the list of stages. If Groups is declared use those; otherwise
	// treat the flat resource fields as a single implicit group.
	stages := t.Groups
	if len(stages) == 0 {
		stages = []types.HookTemplates{*t}
	}

	// deadline is shared across all stages — timeout applies to the entire sequence.
	deadline := time.Now().Add(timeout)

	for i, stage := range stages {
		sl := log.With().Int("stage", i+1).Int("total_stages", len(stages))
		if stage.Name != "" {
			sl = sl.Str("name", stage.Name)
		}
		stageLog := sl.Logger()
		stageLog.Info().Msg("ordered delete: processing stage")

		if !types.EvaluateConditions(resolver.Data(), stage.When, stage.Or, resolver.TemplateEvaluator()) {
			stageLog.Debug().Msg("ordered delete: stage conditions not met — skipping")
			continue
		}

		s := stage // capture for closure
		pending, err := r.submitGroupDeletion(ctx, kube, resolver, obj, &s, guard)
		if err != nil {
			return fmt.Errorf("ordered delete stage %d: submit: %w", i+1, err)
		}

		if len(pending) == 0 {
			stageLog.Debug().Msg("ordered delete: stage has no resources, advancing")
			continue
		}

		stageLog.Info().Int("waiting_for", len(pending)).Msg("ordered delete: waiting for resources to be gone")
		if err := waitForDeletionUntil(ctx, kube, pending, deadline); err != nil {
			return fmt.Errorf("ordered delete stage %d: wait: %w", i+1, err)
		}
		stageLog.Info().Msg("ordered delete: stage complete")
	}

	return nil
}

// submitGroupDeletion issues Delete calls for all resources in a HookTemplates
// block and returns the list of resources to poll for.
func (r *Reconciler[PTR]) submitGroupDeletion(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	t *types.HookTemplates,
	guard func(ctx context.Context, obj domain.Object, ns string) bool,
) ([]orderedDeleteEntry, error) {
	ns := obj.GetNamespace()
	dc := kube.DynamicClient()

	propagation := metav1.DeletePropagationForeground
	delOpts := metav1.DeleteOptions{PropagationPolicy: &propagation}

	var pending []orderedDeleteEntry

	expanded := r.expandAllForDelete(resolver, t)
	for _, rd := range expanded {
		targetNS := ns
		if !rd.namespaced {
			targetNS = ""
		}
		if targetNS != "" && guard != nil && !guard(ctx, obj, targetNS) {
			continue
		}
		for _, item := range rd.names {
			itemNS := targetNS
			if item.namespace != "" {
				itemNS = item.namespace
			}
			var err error
			if itemNS != "" {
				// Namespaced
				err = dc.Resource(rd.gvr).Namespace(itemNS).Delete(ctx, item.name, delOpts)
			} else {
				// Cluster-scoped
				err = dc.Resource(rd.gvr).Delete(ctx, item.name, delOpts)
			}
			if err != nil && !runners.IsNotFoundErr(err) {
				return nil, fmt.Errorf("delete %s/%s: %w", rd.gvr.Resource, item.name, err)
			}
			pending = append(pending, orderedDeleteEntry{
				gvr:       rd.gvr,
				namespace: itemNS,
				name:      item.name,
			})
		}
	}
	return pending, nil
}

// namedResource holds the resolved name and optional namespace override.
type namedResource struct {
	name      string
	namespace string
}

// expandedResourceDef pairs a GVR with the list of resource names derived
// from the template block (after forEach expansion).
type expandedResourceDef struct {
	gvr        schema.GroupVersionResource
	namespaced bool
	names      []namedResource
}

// expandAllForDelete resolves template source names for every resource type
// in the HookTemplates block. Only types with entries are included.
func (r *Reconciler[PTR]) expandAllForDelete(
	resolver *template.Resolver,
	t *types.HookTemplates,
) []expandedResourceDef {
	var out []expandedResourceDef

	out = append(out, resolveNames(resolver, deploymentGVR, true, t.Deployments, func(s types.DeploymentTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, statefulSetGVR, true, t.StatefulSets, func(s types.StatefulSetTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, replicaSetGVR, true, t.ReplicaSets, func(s types.ReplicaSetTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, podGVR, true, t.Pods, func(s types.PodTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, serviceGVR, true, t.Services, func(s types.ServiceTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, secretGVR, true, t.Secrets, func(s types.SecretTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, configMapGVR, true, t.ConfigMaps, func(s types.ConfigMapTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, serviceAccountGVR, true, t.ServiceAccounts, func(s types.ServiceAccountTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, roleGVR, true, t.Roles, func(s types.RoleTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, roleBindingGVR, true, t.RoleBindings, func(s types.RoleBindingTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, clusterRoleGVR, false, t.ClusterRoles, func(s types.ClusterRoleTemplateSource) (string, string) { return s.Name, "" })...)
	out = append(out, resolveNames(resolver, clusterRoleBindingGVR, false, t.ClusterRoleBindings, func(s types.ClusterRoleBindingTemplateSource) (string, string) { return s.Name, "" })...)
	out = append(out, resolveNames(resolver, networkPolicyGVR, true, t.NetworkPolicies, func(s types.NetworkPolicyTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, limitRangeGVR, true, t.LimitRanges, func(s types.LimitRangeTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, resourceQuotaGVR, true, t.ResourceQuotas, func(s types.ResourceQuotaTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, jobGVR, true, t.Jobs, func(s types.JobTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, cronJobGVR, true, t.CronJobs, func(s types.CronJobTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, ingressGVR, true, t.Ingresses, func(s types.IngressTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, pvcGVR, true, t.PersistentVolumeClaims, func(s types.PVCTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, pvGVR, false, t.PersistentVolumes, func(s types.PVTemplateSource) (string, string) { return s.Name, "" })...)
	out = append(out, resolveNames(resolver, hpaGVR, true, t.HorizontalPodAutoscalers, func(s types.HPATemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, pdbGVR, true, t.PodDisruptionBudgets, func(s types.PDBTemplateSource) (string, string) { return s.Name, s.Namespace })...)
	out = append(out, resolveNames(resolver, namespaceGVR, false, t.Namespaces, func(s types.NamespaceTemplateSource) (string, string) { return s.Name, "" })...)

	return out
}

// resolveNames is a generic helper that resolves template source names via
// the resolver and returns an expandedResourceDef when any names are found.
func resolveNames[S any](
	resolver *template.Resolver,
	gvr schema.GroupVersionResource,
	namespaced bool,
	sources []S,
	nameNS func(S) (string, string),
) []expandedResourceDef {
	if len(sources) == 0 {
		return nil
	}
	var items []namedResource
	for _, src := range sources {
		rawName, rawNS := nameNS(src)
		name, _ := resolver.Resolve(rawName)
		ns, _ := resolver.Resolve(rawNS)
		if name == "" {
			continue
		}
		items = append(items, namedResource{name: name, namespace: ns})
	}
	if len(items) == 0 {
		return nil
	}
	return []expandedResourceDef{{gvr: gvr, namespaced: namespaced, names: items}}
}

// waitForDeletionUntil polls the API server until all resources in pending are gone
// or the deadline passes. Uses Get (not informer) so the answer is authoritative.
func waitForDeletionUntil(ctx context.Context, kube kubeclient.Interface, pending []orderedDeleteEntry, deadline time.Time) error {
	dc := kube.DynamicClient()

	for time.Now().Before(deadline) {
		var remaining []orderedDeleteEntry
		for _, e := range pending {
			var err error
			if e.namespace != "" {
				_, err = dc.Resource(e.gvr).Namespace(e.namespace).Get(ctx, e.name, metav1.GetOptions{})
			} else {
				_, err = dc.Resource(e.gvr).Get(ctx, e.name, metav1.GetOptions{})
			}
			if err == nil {
				remaining = append(remaining, e) // still exists
			} else if !runners.IsNotFoundErr(err) {
				return fmt.Errorf("polling %s/%s: %w", e.gvr.Resource, e.name, err)
			}
		}
		if len(remaining) == 0 {
			return nil
		}
		pending = remaining

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(orderedDeletePollInterval):
		}
	}

	names := make([]string, 0, len(pending))
	for _, e := range pending {
		names = append(names, e.gvr.Resource+"/"+e.name)
	}
	return fmt.Errorf("timed out waiting for deletion of: %v", names)
}
