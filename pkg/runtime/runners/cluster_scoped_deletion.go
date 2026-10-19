// pkg/runners/cluster_scoped_deletion.go
package runners

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/children"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/core/clusterrolebindings"
	"github.com/intentd/inrun/pkg/resources/core/clusterroles"
	"github.com/intentd/inrun/pkg/resources/core/customresources"
	"github.com/intentd/inrun/pkg/resources/core/namespaces"
	"github.com/intentd/inrun/pkg/resources/core/pvs"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// DeleteOwnedClusterScopedResources explicitly deletes all cluster-scoped resources declared across
// onCreate, onReconcile, and onDelete that are owned by this CR.
//
// Kubernetes GC does not handle this automatically: owner references from
// namespace-scoped resources (CRs) to cluster-scoped resources (Namespaces, ClusterRoles,
// ClusterRoleBindings, PersistentVolumes) are not honoured by the garbage collector.
// Explicit deletion is required.
//
// This should be called during the deletion path of the reconciler.
func DeleteOwnedClusterScopedResources(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	if err := deleteOwnedNamespaces(ctx, kube, resolver, obj, box); err != nil {
		return err
	}
	if err := deleteOwnedClusterRoles(ctx, kube, resolver, obj, box); err != nil {
		return err
	}
	if err := deleteOwnedClusterRoleBindings(ctx, kube, resolver, obj, box); err != nil {
		return err
	}
	if err := deleteOwnedPersistentVolumes(ctx, kube, resolver, obj, box); err != nil {
		return err
	}
	return deleteOwnedCustomResources(ctx, kube, resolver, obj, box)
}

func deleteOwnedNamespaces(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	var srcs []types.NamespaceTemplateSource
	for _, hook := range allHooks(box) {
		if hook != nil {
			srcs = append(srcs, hook.Namespaces...)
		}
	}
	for i, src := range children.ExpandForEachNamespaces(resolver, srcs) {
		name, _ := resolver.Resolve(src.Name)
		if name == "" {
			continue
		}
		if err := namespaces.DeleteIfOwned(ctx, kube.Clientset(), obj, name); err != nil {
			return fmt.Errorf("namespace[%d] %q: %w", i, name, err)
		}
	}
	return nil
}

func deleteOwnedClusterRoles(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	var srcs []types.ClusterRoleTemplateSource
	for _, hook := range allHooks(box) {
		if hook != nil {
			srcs = append(srcs, hook.ClusterRoles...)
		}
	}
	for i, src := range children.ExpandForEachClusterRoles(resolver, srcs) {
		name, _ := resolver.Resolve(src.Name)
		if name == "" {
			continue
		}
		if err := clusterroles.DeleteIfOwned(ctx, kube.Clientset(), obj, name); err != nil {
			return fmt.Errorf("clusterrole[%d] %q: %w", i, name, err)
		}
	}
	return nil
}

func deleteOwnedClusterRoleBindings(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	var srcs []types.ClusterRoleBindingTemplateSource
	for _, hook := range allHooks(box) {
		if hook != nil {
			srcs = append(srcs, hook.ClusterRoleBindings...)
		}
	}
	for i, src := range children.ExpandForEachClusterRoleBindings(resolver, srcs) {
		name, _ := resolver.Resolve(src.Name)
		if name == "" {
			continue
		}
		if err := clusterrolebindings.DeleteIfOwned(ctx, kube.Clientset(), obj, name); err != nil {
			return fmt.Errorf("clusterrolebinding[%d] %q: %w", i, name, err)
		}
	}
	return nil
}

func deleteOwnedPersistentVolumes(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	var srcs []types.PVTemplateSource
	for _, hook := range allHooks(box) {
		if hook != nil {
			srcs = append(srcs, hook.PersistentVolumes...)
		}
	}
	for i, src := range children.ExpandForEachPVs(resolver, srcs) {
		name, _ := resolver.Resolve(src.Name)
		if name == "" {
			continue
		}
		if err := pvs.DeleteIfOwned(ctx, kube.Clientset(), obj, name); err != nil {
			return fmt.Errorf("persistentvolume[%d] %q: %w", i, name, err)
		}
	}
	return nil
}

func deleteOwnedCustomResources(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	obj domain.Object,
	box types.OperatorBoxConfig,
) error {
	var srcs []types.CustomResourceTemplateSource
	for _, hook := range allHooks(box) {
		if hook != nil {
			srcs = append(srcs, hook.CustomResource...)
		}
	}
	for i, src := range children.ExpandForEachCustomResources(resolver, srcs) {
		if src.IsNamespaced() {
			continue // GC handles namespace-scoped custom resources
		}
		name, _ := resolver.Resolve(src.Metadata.Name)
		if name == "" {
			continue
		}
		if err := customresources.DeleteIfOwned(ctx, kube.DynamicClient(), kube.RESTMapper(), obj, name, "", src.APIVersion, src.Kind); err != nil {
			return fmt.Errorf("customresource[%d] %q: %w", i, name, err)
		}
	}
	return nil
}

// allHooks returns all three lifecycle hook blocks in declaration order.
func allHooks(box types.OperatorBoxConfig) []*types.HookTemplates {
	return []*types.HookTemplates{box.EffectiveOnCreate(), box.EffectiveOnReconcile(), box.EffectiveOnDelete()}
}
