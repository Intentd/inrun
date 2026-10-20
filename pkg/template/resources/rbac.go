package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/intentd/kubewright/core/clusterrolebindings"
	"github.com/intentd/kubewright/core/clusterroles"
	"github.com/intentd/kubewright/core/rolebindings"
	"github.com/intentd/kubewright/core/roles"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ResolveRole resolves all template expressions in a RoleTemplateSource.
func ResolveRole(r *template.Resolver, src types.RoleTemplateSource) (types.RoleTemplateSource, error) {
	resolved := types.RoleTemplateSource{
		Reconcile: src.Reconcile,
	}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("role.name: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("role.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("role.labels: %w", err)
	}

	// Resolve resourceNames in each rule (apiGroups/resources/verbs are typically static)
	for _, rule := range src.Rules {
		resolvedRule := types.PolicyRuleSpec{
			APIGroups: rule.APIGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
		}
		for _, rn := range rule.ResourceNames {
			rv, resolveErr := r.Resolve(rn)
			if resolveErr != nil {
				return resolved, fmt.Errorf("role.rules.resourceNames: %w", resolveErr)
			}
			resolvedRule.ResourceNames = append(resolvedRule.ResourceNames, rv)
		}
		resolved.Rules = append(resolved.Rules, resolvedRule)
	}

	return resolved, nil
}

// ResolveRoleBinding resolves all template expressions in a RoleBindingTemplateSource.
func ResolveRoleBinding(r *template.Resolver, src types.RoleBindingTemplateSource) (types.RoleBindingTemplateSource, error) {
	resolved := types.RoleBindingTemplateSource{
		Reconcile: src.Reconcile,
	}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("rolebinding.name: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("rolebinding.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("rolebinding.labels: %w", err)
	}

	resolved.RoleRef.Kind = src.RoleRef.Kind
	if resolved.RoleRef.Name, err = r.Resolve(src.RoleRef.Name); err != nil {
		return resolved, fmt.Errorf("rolebinding.roleRef.name: %w", err)
	}

	for i, s := range src.Subjects {
		rs := types.SubjectSpec{Kind: s.Kind}
		if rs.Name, err = r.Resolve(s.Name); err != nil {
			return resolved, fmt.Errorf("rolebinding.subjects[%d].name: %w", i, err)
		}
		if rs.Namespace, err = r.Resolve(s.Namespace); err != nil {
			return resolved, fmt.Errorf("rolebinding.subjects[%d].namespace: %w", i, err)
		}
		resolved.Subjects = append(resolved.Subjects, rs)
	}

	return resolved, nil
}

// ResolveClusterRole resolves all template expressions in a ClusterRoleTemplateSource.
func ResolveClusterRole(r *template.Resolver, src types.ClusterRoleTemplateSource) (types.ClusterRoleTemplateSource, error) {
	resolved := types.ClusterRoleTemplateSource{
		Reconcile: src.Reconcile,
	}
	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("clusterrole.name: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("clusterrole.labels: %w", err)
	}

	for _, rule := range src.Rules {
		resolvedRule := types.PolicyRuleSpec{
			APIGroups: rule.APIGroups,
			Resources: rule.Resources,
			Verbs:     rule.Verbs,
		}
		for _, rn := range rule.ResourceNames {
			rv, resolveErr := r.Resolve(rn)
			if resolveErr != nil {
				return resolved, fmt.Errorf("clusterrole.rules.resourceNames: %w", resolveErr)
			}
			resolvedRule.ResourceNames = append(resolvedRule.ResourceNames, rv)
		}
		resolved.Rules = append(resolved.Rules, resolvedRule)
	}

	return resolved, nil
}

// ResolveClusterRoleBinding resolves all template expressions in a ClusterRoleBindingTemplateSource.
func ResolveClusterRoleBinding(r *template.Resolver, src types.ClusterRoleBindingTemplateSource) (types.ClusterRoleBindingTemplateSource, error) {
	resolved := types.ClusterRoleBindingTemplateSource{

		Reconcile: src.Reconcile,
	}
	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("clusterrolebinding.name: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("clusterrolebinding.labels: %w", err)
	}

	resolved.RoleRef.Kind = src.RoleRef.Kind
	if resolved.RoleRef.Name, err = r.Resolve(src.RoleRef.Name); err != nil {
		return resolved, fmt.Errorf("clusterrolebinding.roleRef.name: %w", err)
	}

	for i, s := range src.Subjects {
		rs := types.SubjectSpec{Kind: s.Kind}
		if rs.Name, err = r.Resolve(s.Name); err != nil {
			return resolved, fmt.Errorf("clusterrolebinding.subjects[%d].name: %w", i, err)
		}
		if rs.Namespace, err = r.Resolve(s.Namespace); err != nil {
			return resolved, fmt.Errorf("clusterrolebinding.subjects[%d].namespace: %w", i, err)
		}
		resolved.Subjects = append(resolved.Subjects, rs)
	}

	return resolved, nil
}

// ClusterRoleBindingSpec builds a clusterrolebindings.Spec from a ClusterRoleBindingTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func ClusterRoleBindingSpec(src types.ClusterRoleBindingTemplateSource, owner domain.Object) clusterrolebindings.Spec {
	ownerName := owner.GetName()
	spec := clusterrolebindings.Spec{
		Name:   src.Name,
		Labels: make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-crb"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	kind := src.RoleRef.Kind
	if kind == "" {
		kind = "ClusterRole"
	}
	spec.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     kind,
		Name:     src.RoleRef.Name,
	}

	for _, s := range src.Subjects {
		spec.Subjects = append(spec.Subjects, rbacv1.Subject{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// ClusterRoleSpec builds a clusterroles.Spec from a ClusterRoleTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func ClusterRoleSpec(src types.ClusterRoleTemplateSource, owner domain.Object) clusterroles.Spec {
	ownerName := owner.GetName()
	spec := clusterroles.Spec{
		Name:   src.Name,
		Labels: make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-cluster-role"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	for _, r := range src.Rules {
		spec.Rules = append(spec.Rules, rbacv1.PolicyRule{
			APIGroups:     r.APIGroups,
			Resources:     r.Resources,
			Verbs:         r.Verbs,
			ResourceNames: r.ResourceNames,
		})
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// RoleBindingSpec builds a rolebindings.Spec from a RoleBindingTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func RoleBindingSpec(src types.RoleBindingTemplateSource, owner domain.Object) rolebindings.Spec {
	ownerName := owner.GetName()
	spec := rolebindings.Spec{
		Name:      src.Name,
		Namespace: src.Namespace,
		Labels:    make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-rolebinding"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	kind := src.RoleRef.Kind
	if kind == "" {
		kind = "Role"
	}
	spec.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     kind,
		Name:     src.RoleRef.Name,
	}

	for _, s := range src.Subjects {
		spec.Subjects = append(spec.Subjects, rbacv1.Subject{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// RoleSpec builds a roles.Spec from a RoleTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func RoleSpec(src types.RoleTemplateSource, owner domain.Object) roles.Spec {
	ownerName := owner.GetName()
	spec := roles.Spec{
		Name:      src.Name,
		Namespace: src.Namespace,
		Labels:    make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-role"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	for _, r := range src.Rules {
		spec.Rules = append(spec.Rules, rbacv1.PolicyRule{
			APIGroups:     r.APIGroups,
			Resources:     r.Resources,
			Verbs:         r.Verbs,
			ResourceNames: r.ResourceNames,
		})
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}
