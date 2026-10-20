package runners

// Storage resources: PersistentVolumeClaims and PersistentVolumes.

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/core/pvcs"
	"github.com/intentd/kubewright/core/pvs"
)

// RunPVCs writes PersistentVolumeClaim template entries; runChildren describes how.
func RunPVCs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PVCTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, pvcChild)
}

var pvcChild = child[types.PVCTemplateSource, pvcs.Spec]{
	log:      "pvc",
	resource: "PersistentVolumeClaim",
	field:    "persistentVolumeClaims",
	entry: func(s types.PVCTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolvePVC,
	spec:          noPresets(resources.PVCSpec),
	create:        pvcs.Create,
	apply:         pvcs.Apply,
	deleteIfOwned: pvcs.DeleteIfOwned,
}

// RunPVs writes PersistentVolume template entries; runChildren describes how.
func RunPVs(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.PVTemplateSource,
	update bool,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, nil, pvChild)
}

var pvChild = child[types.PVTemplateSource, pvs.Spec]{
	log:      "pv",
	resource: "PersistentVolume",
	field:    "persistentVolumes",
	entry: func(s types.PVTemplateSource) entryFields {
		return entryFields{Name: s.Name, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolvePV,
	spec:          noPresets(resources.PVSpec),
	create:        pvs.Create,
	apply:         pvs.Apply,
	deleteIfOwned: clusterDelete(pvs.DeleteIfOwned),
	clusterScoped: true,
}
