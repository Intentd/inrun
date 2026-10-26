package runners

// Configuration resources: Namespaces, ConfigMaps and ServiceAccounts.
// Secrets, which can be copied, generated once or made for TLS, are in secrets.go.

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/core/configmaps"
	"github.com/intentd/kubewright/core/namespaces"
	"github.com/intentd/kubewright/core/serviceaccounts"
)

// RunNamespaces writes Namespace template entries; runChildren describes how.
//
// Namespaces are only ever created: they have no meaningful drift to correct.
func RunNamespaces(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.NamespaceTemplateSource,
	update bool,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, nil, namespaceChild)
}

var namespaceChild = child[types.NamespaceTemplateSource, namespaces.Spec]{
	log:      "namespace",
	resource: "Namespace",
	field:    "namespaces",
	entry: func(s types.NamespaceTemplateSource) entryFields {
		return entryFields{Name: s.Name, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveNamespace,
	spec:          noPresets(resources.NamespaceSpec),
	create:        namespaces.Create,
	deleteIfOwned: clusterDelete(namespaces.DeleteIfOwned),
	clusterScoped: true,
}

// RunConfigMaps writes ConfigMap template entries; runChildren describes how.
//
// An entry with toNamespaces writes the ConfigMap into each of those namespaces
// instead of the owner's.
func RunConfigMaps(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ConfigMapTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, configmapChild)
}

var configmapChild = child[types.ConfigMapTemplateSource, configmaps.Spec]{
	log:      "configmap",
	resource: "ConfigMap",
	field:    "configMaps",
	entry: func(s types.ConfigMapTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveConfigMap,
	spec:          noPresets(resources.ConfigMapSpec),
	create:        configmaps.Create,
	apply:         configmaps.Apply,
	deleteIfOwned: configmaps.DeleteIfOwned,
	toNamespaces:  func(s types.ConfigMapTemplateSource) []string { return s.ToNamespaces },
	inNamespace:   func(spec configmaps.Spec, ns string) configmaps.Spec { spec.Namespace = ns; return spec },
}

// RunServiceAccounts writes ServiceAccount template entries; runChildren describes how.
//
// ServiceAccounts are only ever created: nothing in one drifts after it exists.
func RunServiceAccounts(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ServiceAccountTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, serviceaccountChild)
}

var serviceaccountChild = child[types.ServiceAccountTemplateSource, serviceaccounts.Spec]{
	log:      "serviceaccount",
	resource: "ServiceAccount",
	field:    "serviceAccounts",
	entry: func(s types.ServiceAccountTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveServiceAccount,
	spec:          noPresets(resources.ServiceAccountSpec),
	create:        serviceaccounts.Create,
	deleteIfOwned: serviceaccounts.DeleteIfOwned,
}
