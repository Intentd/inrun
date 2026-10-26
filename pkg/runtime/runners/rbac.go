package runners

// RBAC resources: Roles, RoleBindings, ClusterRoles and ClusterRoleBindings.

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/core/clusterrolebindings"
	"github.com/intentd/kubewright/core/clusterroles"
	"github.com/intentd/kubewright/core/rolebindings"
	"github.com/intentd/kubewright/core/roles"
)

// RunRoles writes Role template entries; runChildren describes how.
//
// An onCreate entry with reconcile: true is applied directly, without a Create first.
func RunRoles(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.RoleTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, roleChild)
}

var roleChild = child[types.RoleTemplateSource, roles.Spec]{
	log:      "role",
	resource: "Role",
	field:    "roles",
	entry: func(s types.RoleTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:          resources.ResolveRole,
	spec:             noPresets(resources.RoleSpec),
	create:           roles.Create,
	apply:            roles.Apply,
	deleteIfOwned:    roles.DeleteIfOwned,
	applyOnReconcile: true,
}

// RunRoleBindings writes RoleBinding template entries; runChildren describes how.
//
// An onCreate entry with reconcile: true is applied directly, without a Create first.
func RunRoleBindings(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.RoleBindingTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, rolebindingChild)
}

var rolebindingChild = child[types.RoleBindingTemplateSource, rolebindings.Spec]{
	log:      "rolebinding",
	resource: "RoleBinding",
	field:    "roleBindings",
	entry: func(s types.RoleBindingTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:          resources.ResolveRoleBinding,
	spec:             noPresets(resources.RoleBindingSpec),
	create:           rolebindings.Create,
	apply:            rolebindings.Apply,
	deleteIfOwned:    rolebindings.DeleteIfOwned,
	applyOnReconcile: true,
}

// RunClusterRoles writes ClusterRole template entries; runChildren describes how.
func RunClusterRoles(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ClusterRoleTemplateSource,
	update bool,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, nil, clusterroleChild)
}

var clusterroleChild = child[types.ClusterRoleTemplateSource, clusterroles.Spec]{
	log:      "clusterrole",
	resource: "ClusterRole",
	field:    "clusterRoles",
	entry: func(s types.ClusterRoleTemplateSource) entryFields {
		return entryFields{Name: s.Name, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveClusterRole,
	spec:          noPresets(resources.ClusterRoleSpec),
	create:        clusterroles.Create,
	apply:         clusterroles.Apply,
	deleteIfOwned: clusterDelete(clusterroles.DeleteIfOwned),
	clusterScoped: true,
}

// RunClusterRoleBindings writes ClusterRoleBinding template entries; runChildren describes how.
func RunClusterRoleBindings(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ClusterRoleBindingTemplateSource,
	update bool,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, nil, clusterrolebindingChild)
}

var clusterrolebindingChild = child[types.ClusterRoleBindingTemplateSource, clusterrolebindings.Spec]{
	log:      "clusterrolebinding",
	resource: "ClusterRoleBinding",
	field:    "clusterRoleBindings",
	entry: func(s types.ClusterRoleBindingTemplateSource) entryFields {
		return entryFields{Name: s.Name, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveClusterRoleBinding,
	spec:          noPresets(resources.ClusterRoleBindingSpec),
	create:        clusterrolebindings.Create,
	apply:         clusterrolebindings.Apply,
	deleteIfOwned: clusterDelete(clusterrolebindings.DeleteIfOwned),
	clusterScoped: true,
}
