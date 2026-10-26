package runners

// Policy resources: HorizontalPodAutoscalers, PodDisruptionBudgets,
// ResourceQuotas and LimitRanges.

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/core/hpas"
	"github.com/intentd/kubewright/core/limitranges"
	"github.com/intentd/kubewright/core/pdbs"
	"github.com/intentd/kubewright/core/resourcequotas"
)

// RunHPAs writes HorizontalPodAutoscaler template entries; runChildren describes how.
func RunHPAs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.HPATemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, hpaChild)
}

var hpaChild = child[types.HPATemplateSource, hpas.Spec]{
	log:      "hpa",
	resource: "HorizontalPodAutoscaler",
	field:    "hpa",
	entry: func(s types.HPATemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveHPA,
	spec:          resources.HPASpec,
	create:        hpas.Create,
	apply:         hpas.Apply,
	deleteIfOwned: hpas.DeleteIfOwned,
}

// RunPDBs writes PodDisruptionBudget template entries; runChildren describes how.
func RunPDBs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PDBTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, pdbChild)
}

var pdbChild = child[types.PDBTemplateSource, pdbs.Spec]{
	log:      "pdb",
	resource: "PodDisruptionBudget",
	field:    "pdb",
	entry: func(s types.PDBTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolvePDB,
	spec:          resources.PDBSpec,
	create:        pdbs.Create,
	apply:         pdbs.Apply,
	deleteIfOwned: pdbs.DeleteIfOwned,
}

// RunResourceQuotas writes ResourceQuota template entries; runChildren describes how.
//
// An entry with toNamespaces writes the ResourceQuota into each of those
// namespaces instead of the owner's.
func RunResourceQuotas(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ResourceQuotaTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, resourcequotaChild)
}

var resourcequotaChild = child[types.ResourceQuotaTemplateSource, resourcequotas.Spec]{
	log:      "resourcequota",
	resource: "ResourceQuota",
	field:    "resourceQuotas",
	entry: func(s types.ResourceQuotaTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveResourceQuota,
	spec:          resources.ResourceQuotaSpec,
	create:        resourcequotas.Create,
	apply:         resourcequotas.Apply,
	deleteIfOwned: resourcequotas.DeleteIfOwned,
	toNamespaces:  func(s types.ResourceQuotaTemplateSource) []string { return s.ToNamespaces },
	inNamespace:   func(spec resourcequotas.Spec, ns string) resourcequotas.Spec { spec.Namespace = ns; return spec },
}

// RunLimitRanges writes LimitRange template entries; runChildren describes how.
//
// An entry with toNamespaces writes the LimitRange into each of those namespaces
// instead of the owner's.
func RunLimitRanges(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.LimitRangeTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, limitrangeChild)
}

var limitrangeChild = child[types.LimitRangeTemplateSource, limitranges.Spec]{
	log:      "limitrange",
	resource: "LimitRange",
	field:    "limitRanges",
	entry: func(s types.LimitRangeTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveLimitRange,
	spec:          resources.LimitRangeSpec,
	create:        limitranges.Create,
	apply:         limitranges.Apply,
	deleteIfOwned: limitranges.DeleteIfOwned,
	toNamespaces:  func(s types.LimitRangeTemplateSource) []string { return s.ToNamespaces },
	inNamespace:   func(spec limitranges.Spec, ns string) limitranges.Spec { spec.Namespace = ns; return spec },
}
