// pkg/reconciler/run_template_reconcile.go
//
// Execution order (each step can reference all previous steps):
//
//  1. Receive resolver (already enriched with cross, normalized) from PreparedRequest
//  2. runExternal        	      → .external.<n>.status, .body (HTTP calls)
//  3. forEach expand             → N sources from N-element list fields
//  4. onCreate groups            → deployments, services, secrets, configmaps, ...
//  5. onReconcile groups

package generic

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/runtime/runners"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// runTemplateReconcile interprets the Catalog's onCreate and onReconcile blocks.
// Returns the enriched resolver so callers (reconcileImpl) can pass cross/external
// data into patchStatusWithChildren for status field evaluation.
func (r *Reconciler[PTR]) runTemplateReconcile(ctx context.Context, resolver *template.Resolver, obj domain.Object, box types.OperatorBoxConfig) (*template.Resolver, error) {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return resolver, fmt.Errorf("kubeclient not found in context")
	}

	// Step 1: resolver received from PreparedRequest already carries cross data.
	var err error

	// Step 2: external HTTP calls
	if t := box.EffectiveOnReconcile(); t != nil && len(t.External) > 0 {
		resolver, err = runExternal(ctx, r.crd.GVKString(), resolver, t.External, r.kube.Clientset())
		if err != nil {
			return resolver, fmt.Errorf("external calls: %w", err)
		}
	}
	if t := box.EffectiveOnCreate(); t != nil && len(t.External) > 0 {
		resolver, err = runExternal(ctx, r.crd.GVKString(), resolver, t.External, r.kube.Clientset())
		if err != nil {
			return resolver, fmt.Errorf("external calls: %w", err)
		}
	}

	// Step 3: onCreate resource groups (update=false)
	if t := box.EffectiveOnCreate(); t != nil {
		if err := r.runResourceGroup(ctx, kube, resolver, obj, t, false); err != nil {
			return resolver, err
		}
	}

	// Step 4: onReconcile resource groups (update=true)
	if t := box.EffectiveOnReconcile(); t != nil {
		if err := r.runResourceGroup(ctx, kube, resolver, obj, t, true); err != nil {
			return resolver, err
		}
	}

	return resolver, nil
}

// runResourceGroup dispatches all resource types in one HookTemplates block.
// forEach expansion happens here — run_*.go receives already-expanded slices.
func (r *Reconciler[PTR]) runResourceGroup(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	t *types.HookTemplates,
	update bool,
) error {
	if !types.EvaluateConditions(resolver.Data(), t.When, t.Or, resolver.TemplateEvaluator()) {
		return nil
	}

	// Guard closure — captures r for access to CRD config.
	// nil-safe: if CRD has no restrictions, guard is a no-op.
	guard := r.namespaceGuardFunc()

	labelMgr := labels.NewManager(labels.Config{
		Standalone:                r.kat.IsStandaloneGateway(),
		DeletionProtectionEnabled: r.kat.IsDeletionProtectionEnabled(),
	})

	// Create namespaces first
	if err := runners.RunNamespaces(ctx, kube, resolver, obj,
		children.ExpandForEachNamespaces(resolver, t.Namespaces), update); err != nil {
		return err
	}

	if err := runners.RunSecrets(ctx, kube, resolver, obj,
		children.ExpandForEachSecrets(resolver, t.Secrets), update, guard); err != nil {
		return err
	}
	if err := runners.RunConfigMaps(ctx, kube, resolver, obj,
		children.ExpandForEachConfigMaps(resolver, t.ConfigMaps), update, guard); err != nil {
		return err
	}
	if err := runners.RunNetworkPolicies(ctx, kube, resolver, obj,
		children.ExpandForEachNetworkPolicies(resolver, t.NetworkPolicies), update, guard); err != nil {
		return err
	}
	if err := runners.RunResourceQuotas(ctx, kube, resolver, obj,
		children.ExpandForEachResourceQuotas(resolver, t.ResourceQuotas), update, guard); err != nil {
		return err
	}
	if err := runners.RunLimitRanges(ctx, kube, resolver, obj,
		children.ExpandForEachLimitRanges(resolver, t.LimitRanges), update, guard); err != nil {
		return err
	}
	if err := runners.RunClusterRoles(ctx, kube, resolver, obj,
		children.ExpandForEachClusterRoles(resolver, t.ClusterRoles), update); err != nil {
		return err
	}
	if err := runners.RunClusterRoleBindings(ctx, kube, resolver, obj,
		children.ExpandForEachClusterRoleBindings(resolver, t.ClusterRoleBindings), update); err != nil {
		return err
	}
	if err := runners.RunServiceAccounts(ctx, kube, resolver, obj,
		children.ExpandForEachServiceAccounts(resolver, t.ServiceAccounts), update, guard); err != nil {
		return err
	}
	if err := runners.RunRoles(ctx, kube, resolver, obj,
		children.ExpandForEachRoles(resolver, t.Roles), update, guard); err != nil {
		return err
	}
	if err := runners.RunRoleBindings(ctx, kube, resolver, obj,
		children.ExpandForEachRoleBindings(resolver, t.RoleBindings), update, guard); err != nil {
		return err
	}
	if err := runCustomResources(ctx, kube, resolver, obj,
		children.ExpandForEachCustomResources(resolver, t.CustomResource), update, guard, labelMgr,
		r.kat.IsDeletionProtectionEnabled() && r.crd.ShouldProtectCRs()); err != nil {
		return err
	}
	if err := runners.RunReplicaSets(ctx, kube, resolver, obj,
		children.ExpandForEachReplicaSets(resolver, t.ReplicaSets), update, guard); err != nil {
		return err
	}
	if err := runners.RunDeployments(ctx, kube, resolver, obj,
		children.ExpandForEachDeployments(resolver, t.Deployments), update, guard); err != nil {
		return err
	}
	if err := runners.RunServices(ctx, kube, resolver, obj,
		children.ExpandForEachServices(resolver, t.Services), update, guard); err != nil {
		return err
	}
	if err := runners.RunJobs(ctx, kube, resolver, obj,
		children.ExpandForEachJobs(resolver, t.Jobs), guard); err != nil {
		return err
	}
	if err := runners.RunCronJobs(ctx, kube, resolver, obj,
		children.ExpandForEachCronJobs(resolver, t.CronJobs), update, guard); err != nil {
		return err
	}
	if err := runners.RunStatefulSets(ctx, kube, resolver, obj,
		children.ExpandForEachStatefulSets(resolver, t.StatefulSets), update, guard); err != nil {
		return err
	}
	if err := runners.RunPVs(ctx, kube, resolver, obj,
		children.ExpandForEachPVs(resolver, t.PersistentVolumes), update); err != nil {
		return err
	}
	if err := runners.RunPVCs(ctx, kube, resolver, obj,
		children.ExpandForEachPVCs(resolver, t.PersistentVolumeClaims), update, guard); err != nil {
		return err
	}
	if err := runners.RunIngresses(ctx, kube, resolver, obj,
		children.ExpandForEachIngresses(resolver, t.Ingresses), update, guard); err != nil {
		return err
	}
	if err := runners.RunHPAs(ctx, kube, resolver, obj,
		children.ExpandForEachHPAs(resolver, t.HorizontalPodAutoscalers), update, guard); err != nil {
		return err
	}
	if err := runners.RunPDBs(ctx, kube, resolver, obj,
		children.ExpandForEachPDBs(resolver, t.PodDisruptionBudgets), update, guard); err != nil {
		return err
	}
	if err := runners.RunPods(ctx, kube, resolver, obj,
		children.ExpandForEachPods(resolver, t.Pods), update, guard); err != nil {
		return err
	}
	return nil
}

// runTemplateOnDelete interprets the onDelete block.
func (r *Reconciler[PTR]) runTemplateOnDelete(ctx context.Context, resolver *template.Resolver, obj domain.Object, box types.OperatorBoxConfig) error {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return fmt.Errorf("kubeclient not found in context")
	}

	guard := r.namespaceGuardFunc()

	if t := box.EffectiveOnDelete(); t != nil {
		if types.EvaluateConditions(resolver.Data(), t.When, t.Or, resolver.TemplateEvaluator()) {
			if t.Ordered {
				if err := r.runOrderedDelete(ctx, kube, resolver, obj, t, guard); err != nil {
					return err
				}
			} else {
				timeout := runners.DefaultDeleteJobTimeout
				if t.Timeout != nil && t.Timeout.Duration > 0 {
					timeout = t.Timeout.Duration
				}
				if err := runners.RunDeleteJobs(ctx, kube, resolver, obj,
					children.ExpandForEachJobs(resolver, t.Jobs), guard,
					time.Now().Add(timeout)); err != nil {
					return err
				}
			}
		}
	}

	// Cluster-scoped resources cannot have namespace-scoped owners, so GC never cleans them up.
	// Always run explicit cleanup regardless of ordered/unordered path.
	if err := runners.DeleteOwnedClusterScopedResources(ctx, kube, resolver, obj, box); err != nil {
		return fmt.Errorf("cluster-scoped resource cleanup: %w", err)
	}

	return nil
}
