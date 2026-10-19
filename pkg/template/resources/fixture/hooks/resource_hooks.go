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
	}, obj)
	if _, err := namespaces.Apply(ctx, kube.Clientset(), obj, nsSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
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
	}, obj)
	if _, err := configmaps.Apply(ctx, kube.Clientset(), obj, cmSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("configmap: %w", err)
	}

	// ── Secret ────────────────────────────────────────────────────────────────
	secretSpec := resources.SecretSpec(types.SecretTemplateSource{
		Name:      obj.Name + "-creds",
		Namespace: ns,
		Data: map[string]string{
			"token": "probe-token",
		},
	}, obj)
	if _, err := secrets.Apply(ctx, kube.Clientset(), obj, secretSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("secret: %w", err)
	}

	// ── ServiceAccount (Create — no Apply) ───────────────────────────────────
	saSpec := resources.ServiceAccountSpec(types.ServiceAccountTemplateSource{
		Name:      obj.Name + "-sa",
		Namespace: ns,
	}, obj)
	if _, err := serviceaccounts.Create(ctx, kube.Clientset(), obj, saSpec); err != nil {
		return domain.Result{}, fmt.Errorf("serviceaccount: %w", err)
	}

	// ── ClusterRole ───────────────────────────────────────────────────────────
	crSpec := resources.ClusterRoleSpec(types.ClusterRoleTemplateSource{
		Name: obj.Name + "-cr",
		Rules: []types.PolicyRuleSpec{
			{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list", "watch"}},
		},
	}, obj)
	if _, err := clusterroles.Apply(ctx, kube.Clientset(), obj, crSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
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
	}, obj)
	if _, err := clusterrolebindings.Apply(ctx, kube.Clientset(), obj, crbSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("clusterrolebinding: %w", err)
	}

	// ── Role ──────────────────────────────────────────────────────────────────
	roleSpec := resources.RoleSpec(types.RoleTemplateSource{
		Name:      obj.Name + "-role",
		Namespace: ns,
		Rules: []types.PolicyRuleSpec{
			{APIGroups: []string{""}, Resources: []string{"configmaps", "secrets"}, Verbs: []string{"get", "list"}},
		},
	}, obj)
	if _, err := roles.Apply(ctx, kube.Clientset(), obj, roleSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("role: %w", err)
	}

	// ── RoleBinding ───────────────────────────────────────────────────────────
	rbSpec := resources.RoleBindingSpec(types.RoleBindingTemplateSource{
		Name:      obj.Name + "-rb",
		Namespace: ns,
		RoleRef:   types.RoleRefSpec{Kind: "Role", Name: obj.Name + "-role"},
		Subjects:  []types.SubjectSpec{{Kind: "ServiceAccount", Name: obj.Name + "-sa", Namespace: ns}},
	}, obj)
	if _, err := rolebindings.Apply(ctx, kube.Clientset(), obj, rbSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("rolebinding: %w", err)
	}

	// ── NetworkPolicy ─────────────────────────────────────────────────────────
	netpolSpec := resources.NetworkPolicySpec(types.NetworkPolicyTemplateSource{
		Name:      obj.Name + "-netpol",
		Namespace: ns,
	}, obj, reg)
	if _, err := networkpolicies.Apply(ctx, kube.Clientset(), obj, netpolSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("networkpolicy: %w", err)
	}

	// ── ResourceQuota ─────────────────────────────────────────────────────────
	quotaSpec := resources.ResourceQuotaSpec(types.ResourceQuotaTemplateSource{
		Name:      obj.Name + "-quota",
		Namespace: ns,
		Hard:      map[string]string{"cpu": "4", "memory": "8Gi", "pods": "20"},
	}, obj, reg)
	if _, err := resourcequotas.Apply(ctx, kube.Clientset(), obj, quotaSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
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
	}, obj, reg)
	if _, err := limitranges.Apply(ctx, kube.Clientset(), obj, lrSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
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
	}, obj, reg)
	if _, err := deployments.Apply(ctx, kube.Clientset(), obj, deploySpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("deployment: %w", err)
	}

	// ── StatefulSet ───────────────────────────────────────────────────────────
	stsSpec := resources.StatefulSetSpec(types.StatefulSetTemplateSource{
		Name:      obj.Name + "-sts",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Replicas:  obj.Spec.Replicas,
		Port:      obj.Spec.Port,
	}, obj, reg)
	if _, err := statefulsets.Apply(ctx, kube.Clientset(), obj, stsSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("statefulset: %w", err)
	}

	// ── ReplicaSet ────────────────────────────────────────────────────────────
	rsSpec := resources.ReplicaSetSpec(types.ReplicaSetTemplateSource{
		Name:      obj.Name + "-rs",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Replicas:  obj.Spec.Replicas,
		Port:      obj.Spec.Port,
	}, obj, reg)
	if _, err := replicasets.Apply(ctx, kube.Clientset(), obj, rsSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("replicaset: %w", err)
	}

	// ── Pod ───────────────────────────────────────────────────────────────────
	podSpec := resources.PodSpec(types.PodTemplateSource{
		Name:      obj.Name + "-pod",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Port:      obj.Spec.Port,
	}, obj, reg)
	if _, err := pods.Apply(ctx, kube.Clientset(), obj, podSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("pod: %w", err)
	}

	// ── Job (Create — no Apply) ──────────────────────────────────────────────
	jobSpec := resources.JobSpec(types.JobTemplateSource{
		Name:      obj.Name + "-init",
		Namespace: ns,
		Image:     "alpine:3.19",
		Command:   []string{"/bin/sh", "-c", "echo probe-job-ok"},
	}, 3, obj, reg)
	if _, err := jobs.Create(ctx, kube.Clientset(), obj, jobSpec); err != nil {
		return domain.Result{}, fmt.Errorf("job: %w", err)
	}

	// ── CronJob ───────────────────────────────────────────────────────────────
	cronSpec := resources.CronJobSpec(types.CronJobTemplateSource{
		Name:      obj.Name + "-cron",
		Namespace: ns,
		Schedule:  obj.Spec.Schedule,
		Image:     "alpine:3.19",
		Command:   []string{"/bin/sh", "-c", "echo probe-cron-ok"},
	}, obj, reg)
	if _, err := cronjobs.Apply(ctx, kube.Clientset(), obj, cronSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("cronjob: %w", err)
	}

	// ── Service ───────────────────────────────────────────────────────────────
	svcSpec := resources.ServiceSpec(types.ServiceTemplateSource{
		Name:       obj.Name + "-svc",
		Namespace:  ns,
		Port:       "80",
		TargetPort: obj.Spec.Port,
	}, obj)
	if _, err := services.Apply(ctx, kube.Clientset(), obj, svcSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("service: %w", err)
	}

	// ── Ingress ───────────────────────────────────────────────────────────────
	ingSpec := resources.IngressSpec(types.IngressTemplateSource{
		Name:        obj.Name + "-ing",
		Namespace:   ns,
		Host:        obj.Name + ".probe.local",
		ServiceName: obj.Name + "-svc",
		ServicePort: "80",
	}, obj)
	if _, err := ingresses.Apply(ctx, kube.Clientset(), obj, ingSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
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
	}, obj, reg)
	if _, err := hpas.Apply(ctx, kube.Clientset(), obj, hpaSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("hpa: %w", err)
	}

	// ── PDB ───────────────────────────────────────────────────────────────────
	pdbSpec := resources.PDBSpec(types.PDBTemplateSource{
		Name:         obj.Name + "-pdb",
		Namespace:    ns,
		Selector:     map[string]string{"app": obj.Name + "-deploy"},
		MinAvailable: "1",
	}, obj, reg)
	if _, err := pdbs.Apply(ctx, kube.Clientset(), obj, pdbSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("pdb: %w", err)
	}

	// ── PV ────────────────────────────────────────────────────────────────────
	pvSpec := resources.PVSpec(types.PVTemplateSource{
		Name:        obj.Name + "-pv",
		Capacity:    obj.Spec.Storage,
		AccessModes: []string{"ReadWriteOnce"},
		HostPath:    "/tmp/inrun-" + obj.Name,
	}, obj)
	if _, err := pvs.Apply(ctx, kube.Clientset(), obj, pvSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("pv: %w", err)
	}

	// ── PVC ───────────────────────────────────────────────────────────────────
	pvcSpec := resources.PVCSpec(types.PVCTemplateSource{
		Name:        obj.Name + "-pvc",
		Namespace:   ns,
		Storage:     obj.Spec.Storage,
		AccessModes: []string{"ReadWriteOnce"},
		VolumeName:  obj.Name + "-pv",
	}, obj)
	if _, err := pvcs.Apply(ctx, kube.Clientset(), obj, pvcSpec, kubeclient.ApplyOptions(kube, nil)); err != nil {
		return domain.Result{}, fmt.Errorf("pvc: %w", err)
	}

	return domain.Result{}, nil
}

// onDelete is a no-op: ResourceProbe is cluster-scoped, so owner references work
// for all resources (namespaced in probe-ns and cluster-scoped). GC handles cleanup.
func onDelete(_ context.Context, _ domain.Request, _ *rpv1alpha1.ResourceProbe) (domain.Result, error) {
	return domain.Result{}, nil
}
