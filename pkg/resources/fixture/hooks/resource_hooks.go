// hooks/resource_hooks.go
//
// Typed Go hooks for the ResourceProbe CRD (group: resources.inrun.dev).
//
// Purpose: exercise every pkg/resources/* Resolve() + Apply() (or Create())
// code path in a single reconcile loop.
//
// Coverage map:
//
//	namespaces, configmaps, secrets, serviceaccounts (Create), clusterroles,
//	clusterrolebindings, roles, rolebindings, networkpolicies, resourcequotas,
//	limitranges, deployments, statefulsets, replicasets, pods, jobs (Create),
//	cronjobs, services, ingresses, hpas, pdbs, pvcs, pvs
package hooks

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/template/resources"

	rpv1alpha1 "github.com/intentd/inrun-resource-probe/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/core/clusterrolebindings"
	"github.com/intentd/inrun/pkg/resources/core/clusterroles"
	"github.com/intentd/inrun/pkg/resources/core/configmaps"
	"github.com/intentd/inrun/pkg/resources/core/cronjobs"
	"github.com/intentd/inrun/pkg/resources/core/deployments"
	"github.com/intentd/inrun/pkg/resources/core/hpas"
	"github.com/intentd/inrun/pkg/resources/core/ingresses"
	"github.com/intentd/inrun/pkg/resources/core/jobs"
	"github.com/intentd/inrun/pkg/resources/core/limitranges"
	"github.com/intentd/inrun/pkg/resources/core/namespaces"
	"github.com/intentd/inrun/pkg/resources/core/networkpolicies"
	"github.com/intentd/inrun/pkg/resources/core/pdbs"
	"github.com/intentd/inrun/pkg/resources/core/pods"
	"github.com/intentd/inrun/pkg/resources/core/pvcs"
	"github.com/intentd/inrun/pkg/resources/core/pvs"
	"github.com/intentd/inrun/pkg/resources/core/replicasets"
	"github.com/intentd/inrun/pkg/resources/core/resourcequotas"
	"github.com/intentd/inrun/pkg/resources/core/rolebindings"
	"github.com/intentd/inrun/pkg/resources/core/roles"
	"github.com/intentd/inrun/pkg/resources/core/secrets"
	"github.com/intentd/inrun/pkg/resources/core/serviceaccounts"
	"github.com/intentd/inrun/pkg/resources/core/services"
	"github.com/intentd/inrun/pkg/resources/core/statefulsets"
	"github.com/intentd/inrun/pkg/types"
)

// ResourceProbeHooks returns the hook implementation for the ResourceProbe CRD.
// Registered in the Blueprint under operatorBox.reconciler.hooks.function.
func ResourceProbeHooks() domain.AnyReconcileHooks {
	return domain.ReconcileHooks[*rpv1alpha1.ResourceProbe]{
		OnReconcile: onReconcile,
		OnDelete:    onDelete,
	}
}

func onReconcile(ctx context.Context, req domain.Request, obj *rpv1alpha1.ResourceProbe) (domain.Result, error) {
	kube, ok := kubeclient.FromContext(ctx)
	if !ok {
		return domain.Result{}, fmt.Errorf("kubeclient not in context")
	}

	var reg *types.PresetRegistry
	ns := obj.Name + "-ns"

	// ── Namespace ─────────────────────────────────────────────────────────────
	nsSpec := resources.NamespaceSpec(types.NamespaceTemplateSource{
		Name: ns,
	}, obj.Name)
	if err := namespaces.Apply(ctx, kube, obj, nsSpec); err != nil {
		return domain.Result{}, fmt.Errorf("namespace: %w", err)
	}

	// ── ConfigMap ─────────────────────────────────────────────────────────────
	cmSpec := resources.ConfigMapSpec(types.ConfigMapTemplateSource{
		Name:      obj.Name + "-config",
		Namespace: ns,
		Data: map[string]string{
			"IMAGE":    obj.Spec.Image,
			"PORT":     obj.Spec.Port,
			"REPLICAS": obj.Spec.Replicas,
		},
	}, obj.Name)
	if err := configmaps.Apply(ctx, kube, obj, cmSpec); err != nil {
		return domain.Result{}, fmt.Errorf("configmap: %w", err)
	}

	// ── Secret ────────────────────────────────────────────────────────────────
	secretSpec := resources.SecretSpec(types.SecretTemplateSource{
		Name:      obj.Name + "-creds",
		Namespace: ns,
		Data: map[string]string{
			"token": "probe-token",
		},
	}, obj.Name)
	if err := secrets.Apply(ctx, kube, obj, secretSpec); err != nil {
		return domain.Result{}, fmt.Errorf("secret: %w", err)
	}

	// ── ServiceAccount (Create — no Apply) ───────────────────────────────────
	saSpec := resources.ServiceAccountSpec(types.ServiceAccountTemplateSource{
		Name:      obj.Name + "-sa",
		Namespace: ns,
	}, obj.Name)
	if err := serviceaccounts.Create(ctx, kube, obj, saSpec); err != nil {
		return domain.Result{}, fmt.Errorf("serviceaccount: %w", err)
	}

	// ── ClusterRole ───────────────────────────────────────────────────────────
	crSpec := resources.ClusterRoleSpec(types.ClusterRoleTemplateSource{
		Name: obj.Name + "-cr",
		Rules: []types.PolicyRuleSpec{
			{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list", "watch"}},
		},
	}, obj.Name)
	if err := clusterroles.Apply(ctx, kube, obj, crSpec); err != nil {
		return domain.Result{}, fmt.Errorf("clusterrole: %w", err)
	}

	// ── ClusterRoleBinding ────────────────────────────────────────────────────
	crbSpec := resources.ClusterRoleBindingSpec(types.ClusterRoleBindingTemplateSource{
		Name: obj.Name + "-crb",
		RoleRef: types.RoleRefSpec{
			Kind: "ClusterRole",
			Name: obj.Name + "-cr",
		},
		Subjects: []types.SubjectSpec{
			{Kind: "ServiceAccount", Name: obj.Name + "-sa", Namespace: ns},
		},
	}, obj.Name)
	if err := clusterrolebindings.Apply(ctx, kube, obj, crbSpec); err != nil {
		return domain.Result{}, fmt.Errorf("clusterrolebinding: %w", err)
	}

	// ── Role ──────────────────────────────────────────────────────────────────
	roleSpec := resources.RoleSpec(types.RoleTemplateSource{
		Name:      obj.Name + "-role",
		Namespace: ns,
		Rules: []types.PolicyRuleSpec{
			{APIGroups: []string{""}, Resources: []string{"configmaps", "secrets"}, Verbs: []string{"get", "list"}},
		},
	}, obj.Name)
	if err := roles.Apply(ctx, kube, obj, roleSpec); err != nil {
		return domain.Result{}, fmt.Errorf("role: %w", err)
	}

	// ── RoleBinding ───────────────────────────────────────────────────────────
	rbSpec := resources.RoleBindingSpec(types.RoleBindingTemplateSource{
		Name:      obj.Name + "-rb",
		Namespace: ns,
		RoleRef:   types.RoleRefSpec{Kind: "Role", Name: obj.Name + "-role"},
		Subjects:  []types.SubjectSpec{{Kind: "ServiceAccount", Name: obj.Name + "-sa", Namespace: ns}},
	}, obj.Name)
	if err := rolebindings.Apply(ctx, kube, obj, rbSpec); err != nil {
		return domain.Result{}, fmt.Errorf("rolebinding: %w", err)
	}

	// ── NetworkPolicy ─────────────────────────────────────────────────────────
	netpolSpec := resources.NetworkPolicySpec(types.NetworkPolicyTemplateSource{
		Name:      obj.Name + "-netpol",
		Namespace: ns,
	}, obj.Name, reg)
	if err := networkpolicies.Apply(ctx, kube, obj, netpolSpec); err != nil {
		return domain.Result{}, fmt.Errorf("networkpolicy: %w", err)
	}

	// ── ResourceQuota ─────────────────────────────────────────────────────────
	quotaSpec := resources.ResourceQuotaSpec(types.ResourceQuotaTemplateSource{
		Name:      obj.Name + "-quota",
		Namespace: ns,
		Hard:      map[string]string{"cpu": "4", "memory": "8Gi", "pods": "20"},
	}, obj.Name, reg)
	if err := resourcequotas.Apply(ctx, kube, obj, quotaSpec); err != nil {
		return domain.Result{}, fmt.Errorf("resourcequota: %w", err)
	}

	// ── LimitRange ────────────────────────────────────────────────────────────
	lrSpec := resources.LimitRangeSpec(types.LimitRangeTemplateSource{
		Name:      obj.Name + "-limits",
		Namespace: ns,
		Limits: []types.LimitRangeItem{
			{
				Type:           "Container",
				Default:        map[string]string{"cpu": "500m", "memory": "512Mi"},
				DefaultRequest: map[string]string{"cpu": "250m", "memory": "256Mi"},
			},
		},
	}, obj.Name, reg)
	if err := limitranges.Apply(ctx, kube, obj, lrSpec); err != nil {
		return domain.Result{}, fmt.Errorf("limitrange: %w", err)
	}

	// ── Deployment ────────────────────────────────────────────────────────────
	deploySpec := resources.DeploymentSpec(types.DeploymentTemplateSource{
		Name:               obj.Name + "-deploy",
		Namespace:          ns,
		Image:              obj.Spec.Image,
		Replicas:           obj.Spec.Replicas,
		Port:               obj.Spec.Port,
		ServiceAccountName: obj.Name + "-sa",
	}, obj.Name, reg)
	if err := deployments.Apply(ctx, kube, obj, deploySpec); err != nil {
		return domain.Result{}, fmt.Errorf("deployment: %w", err)
	}

	// ── StatefulSet ───────────────────────────────────────────────────────────
	stsSpec := resources.StatefulSetSpec(types.StatefulSetTemplateSource{
		Name:      obj.Name + "-sts",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Replicas:  obj.Spec.Replicas,
		Port:      obj.Spec.Port,
	}, obj.Name, reg)
	if err := statefulsets.Apply(ctx, kube, obj, stsSpec); err != nil {
		return domain.Result{}, fmt.Errorf("statefulset: %w", err)
	}

	// ── ReplicaSet ────────────────────────────────────────────────────────────
	rsSpec := resources.ReplicaSetSpec(types.ReplicaSetTemplateSource{
		Name:      obj.Name + "-rs",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Replicas:  obj.Spec.Replicas,
		Port:      obj.Spec.Port,
	}, obj.Name, reg)
	if err := replicasets.Apply(ctx, kube, obj, rsSpec); err != nil {
		return domain.Result{}, fmt.Errorf("replicaset: %w", err)
	}

	// ── Pod ───────────────────────────────────────────────────────────────────
	podSpec := resources.PodSpec(types.PodTemplateSource{
		Name:      obj.Name + "-pod",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Port:      obj.Spec.Port,
	}, obj.Name, reg)
	if err := pods.Apply(ctx, kube, obj, podSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pod: %w", err)
	}

	// ── Job (Create — no Apply) ──────────────────────────────────────────────
	jobSpec := resources.JobSpec(types.JobTemplateSource{
		Name:      obj.Name + "-init",
		Namespace: ns,
		Image:     "alpine:3.19",
		Command:   []string{"/bin/sh", "-c", "echo probe-job-ok"},
	}, 3, obj.Name, reg)
	if err := jobs.Create(ctx, kube, obj, jobSpec); err != nil {
		return domain.Result{}, fmt.Errorf("job: %w", err)
	}

	// ── CronJob ───────────────────────────────────────────────────────────────
	cronSpec := resources.CronJobSpec(types.CronJobTemplateSource{
		Name:      obj.Name + "-cron",
		Namespace: ns,
		Schedule:  obj.Spec.Schedule,
		Image:     "alpine:3.19",
		Command:   []string{"/bin/sh", "-c", "echo probe-cron-ok"},
	}, obj.Name, reg)
	if err := cronjobs.Apply(ctx, kube, obj, cronSpec); err != nil {
		return domain.Result{}, fmt.Errorf("cronjob: %w", err)
	}

	// ── Service ───────────────────────────────────────────────────────────────
	svcSpec := resources.ServiceSpec(types.ServiceTemplateSource{
		Name:       obj.Name + "-svc",
		Namespace:  ns,
		Port:       "80",
		TargetPort: obj.Spec.Port,
	}, obj.Name)
	if err := services.Apply(ctx, kube, obj, svcSpec); err != nil {
		return domain.Result{}, fmt.Errorf("service: %w", err)
	}

	// ── Ingress ───────────────────────────────────────────────────────────────
	ingSpec := resources.IngressSpec(types.IngressTemplateSource{
		Name:        obj.Name + "-ing",
		Namespace:   ns,
		Host:        obj.Name + ".probe.local",
		ServiceName: obj.Name + "-svc",
		ServicePort: "80",
	}, obj.Name)
	if err := ingresses.Apply(ctx, kube, obj, ingSpec); err != nil {
		return domain.Result{}, fmt.Errorf("ingress: %w", err)
	}

	// ── HPA ───────────────────────────────────────────────────────────────────
	hpaSpec := resources.HPASpec(types.HPATemplateSource{
		Name:      obj.Name + "-hpa",
		Namespace: ns,
		ScaleTargetRef: types.ScaleTargetRef{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       obj.Name + "-deploy",
		},
		MinReplicas: "1",
		MaxReplicas: "3",
	}, obj.Name, reg)
	if err := hpas.Apply(ctx, kube, obj, hpaSpec); err != nil {
		return domain.Result{}, fmt.Errorf("hpa: %w", err)
	}

	// ── PDB ───────────────────────────────────────────────────────────────────
	pdbSpec := resources.PDBSpec(types.PDBTemplateSource{
		Name:         obj.Name + "-pdb",
		Namespace:    ns,
		Selector:     map[string]string{"app": obj.Name + "-deploy"},
		MinAvailable: "1",
	}, obj.Name, reg)
	if err := pdbs.Apply(ctx, kube, obj, pdbSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pdb: %w", err)
	}

	// ── PV ────────────────────────────────────────────────────────────────────
	pvSpec := resources.PVSpec(types.PVTemplateSource{
		Name:        obj.Name + "-pv",
		Capacity:    obj.Spec.Storage,
		AccessModes: []string{"ReadWriteOnce"},
		HostPath:    "/tmp/inrun-" + obj.Name,
	}, obj.Name)
	if err := pvs.Apply(ctx, kube, obj, pvSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pv: %w", err)
	}

	// ── PVC ───────────────────────────────────────────────────────────────────
	pvcSpec := resources.PVCSpec(types.PVCTemplateSource{
		Name:        obj.Name + "-pvc",
		Namespace:   ns,
		Storage:     obj.Spec.Storage,
		AccessModes: []string{"ReadWriteOnce"},
		VolumeName:  obj.Name + "-pv",
	}, obj.Name)
	if err := pvcs.Apply(ctx, kube, obj, pvcSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pvc: %w", err)
	}

	return domain.Result{}, nil
}

// onDelete is a no-op: ResourceProbe is cluster-scoped, so owner references work
// for all resources (namespaced in probe-ns and cluster-scoped). GC handles cleanup.
func onDelete(_ context.Context, _ domain.Request, _ *rpv1alpha1.ResourceProbe) (domain.Result, error) {
	return domain.Result{}, nil
}
