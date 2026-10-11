// hooks/resource_hooks.go
//
// Typed Go hooks for the ResourceProbe CRD (group: resources.inrun.dev).
//
// Purpose: exercise every pkg/resources/* Resolve() + Update() (or Create())
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

	rpv1alpha1 "github.com/intentd/inrun-resource-probe/api/v1alpha1"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/resources/clusterrolebindings"
	"github.com/intentd/inrun/pkg/resources/clusterroles"
	"github.com/intentd/inrun/pkg/resources/configmaps"
	"github.com/intentd/inrun/pkg/resources/cronjobs"
	"github.com/intentd/inrun/pkg/resources/deployments"
	"github.com/intentd/inrun/pkg/resources/hpas"
	"github.com/intentd/inrun/pkg/resources/ingresses"
	"github.com/intentd/inrun/pkg/resources/jobs"
	"github.com/intentd/inrun/pkg/resources/limitranges"
	"github.com/intentd/inrun/pkg/resources/namespaces"
	"github.com/intentd/inrun/pkg/resources/networkpolicies"
	"github.com/intentd/inrun/pkg/resources/pdbs"
	"github.com/intentd/inrun/pkg/resources/pods"
	"github.com/intentd/inrun/pkg/resources/pvcs"
	"github.com/intentd/inrun/pkg/resources/pvs"
	"github.com/intentd/inrun/pkg/resources/replicasets"
	"github.com/intentd/inrun/pkg/resources/resourcequotas"
	"github.com/intentd/inrun/pkg/resources/rolebindings"
	"github.com/intentd/inrun/pkg/resources/roles"
	"github.com/intentd/inrun/pkg/resources/secrets"
	"github.com/intentd/inrun/pkg/resources/serviceaccounts"
	"github.com/intentd/inrun/pkg/resources/services"
	"github.com/intentd/inrun/pkg/resources/statefulsets"
	"github.com/intentd/inrun/pkg/types"
)

// ResourceProbeHooks returns the hook implementation for the ResourceProbe CRD.
// Registered in the Catalog under operatorBox.reconciler.hooks.function.
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

	var reg *types.ProfileRegistry
	ns := obj.Name + "-ns"

	// ── Namespace ─────────────────────────────────────────────────────────────
	nsSpec := namespaces.Resolve(types.NamespaceTemplateSource{
		Name: ns,
	}, obj.Name)
	if err := namespaces.Update(ctx, kube, obj, nsSpec); err != nil {
		return domain.Result{}, fmt.Errorf("namespace: %w", err)
	}

	// ── ConfigMap ─────────────────────────────────────────────────────────────
	cmSpec := configmaps.Resolve(types.ConfigMapTemplateSource{
		Name:      obj.Name + "-config",
		Namespace: ns,
		Data: map[string]string{
			"IMAGE":    obj.Spec.Image,
			"PORT":     obj.Spec.Port,
			"REPLICAS": obj.Spec.Replicas,
		},
	}, obj.Name)
	if err := configmaps.Update(ctx, kube, obj, cmSpec); err != nil {
		return domain.Result{}, fmt.Errorf("configmap: %w", err)
	}

	// ── Secret ────────────────────────────────────────────────────────────────
	secretSpec := secrets.Resolve(types.SecretTemplateSource{
		Name:      obj.Name + "-creds",
		Namespace: ns,
		Data: map[string]string{
			"token": "probe-token",
		},
	}, obj.Name)
	if err := secrets.Update(ctx, kube, obj, secretSpec); err != nil {
		return domain.Result{}, fmt.Errorf("secret: %w", err)
	}

	// ── ServiceAccount (Create — no Update) ───────────────────────────────────
	saSpec := serviceaccounts.Resolve(types.ServiceAccountTemplateSource{
		Name:      obj.Name + "-sa",
		Namespace: ns,
	}, obj.Name)
	if err := serviceaccounts.Create(ctx, kube, obj, saSpec); err != nil {
		return domain.Result{}, fmt.Errorf("serviceaccount: %w", err)
	}

	// ── ClusterRole ───────────────────────────────────────────────────────────
	crSpec := clusterroles.Resolve(types.ClusterRoleTemplateSource{
		Name: obj.Name + "-cr",
		Rules: []types.PolicyRuleSpec{
			{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list", "watch"}},
		},
	}, obj.Name)
	if err := clusterroles.Update(ctx, kube, obj, crSpec); err != nil {
		return domain.Result{}, fmt.Errorf("clusterrole: %w", err)
	}

	// ── ClusterRoleBinding ────────────────────────────────────────────────────
	crbSpec := clusterrolebindings.Resolve(types.ClusterRoleBindingTemplateSource{
		Name: obj.Name + "-crb",
		RoleRef: types.RoleRefSpec{
			Kind: "ClusterRole",
			Name: obj.Name + "-cr",
		},
		Subjects: []types.SubjectSpec{
			{Kind: "ServiceAccount", Name: obj.Name + "-sa", Namespace: ns},
		},
	}, obj.Name)
	if err := clusterrolebindings.Update(ctx, kube, obj, crbSpec); err != nil {
		return domain.Result{}, fmt.Errorf("clusterrolebinding: %w", err)
	}

	// ── Role ──────────────────────────────────────────────────────────────────
	roleSpec := roles.Resolve(types.RoleTemplateSource{
		Name:      obj.Name + "-role",
		Namespace: ns,
		Rules: []types.PolicyRuleSpec{
			{APIGroups: []string{""}, Resources: []string{"configmaps", "secrets"}, Verbs: []string{"get", "list"}},
		},
	}, obj.Name)
	if err := roles.Update(ctx, kube, obj, roleSpec); err != nil {
		return domain.Result{}, fmt.Errorf("role: %w", err)
	}

	// ── RoleBinding ───────────────────────────────────────────────────────────
	rbSpec := rolebindings.Resolve(types.RoleBindingTemplateSource{
		Name:      obj.Name + "-rb",
		Namespace: ns,
		RoleRef:   types.RoleRefSpec{Kind: "Role", Name: obj.Name + "-role"},
		Subjects:  []types.SubjectSpec{{Kind: "ServiceAccount", Name: obj.Name + "-sa", Namespace: ns}},
	}, obj.Name)
	if err := rolebindings.Update(ctx, kube, obj, rbSpec); err != nil {
		return domain.Result{}, fmt.Errorf("rolebinding: %w", err)
	}

	// ── NetworkPolicy ─────────────────────────────────────────────────────────
	netpolSpec := networkpolicies.Resolve(types.NetworkPolicyTemplateSource{
		Name:      obj.Name + "-netpol",
		Namespace: ns,
	}, obj.Name, reg)
	if err := networkpolicies.Update(ctx, kube, obj, netpolSpec); err != nil {
		return domain.Result{}, fmt.Errorf("networkpolicy: %w", err)
	}

	// ── ResourceQuota ─────────────────────────────────────────────────────────
	quotaSpec := resourcequotas.Resolve(types.ResourceQuotaTemplateSource{
		Name:      obj.Name + "-quota",
		Namespace: ns,
		Hard:      map[string]string{"cpu": "4", "memory": "8Gi", "pods": "20"},
	}, obj.Name, reg)
	if err := resourcequotas.Update(ctx, kube, obj, quotaSpec); err != nil {
		return domain.Result{}, fmt.Errorf("resourcequota: %w", err)
	}

	// ── LimitRange ────────────────────────────────────────────────────────────
	lrSpec := limitranges.Resolve(types.LimitRangeTemplateSource{
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
	if err := limitranges.Update(ctx, kube, obj, lrSpec); err != nil {
		return domain.Result{}, fmt.Errorf("limitrange: %w", err)
	}

	// ── Deployment ────────────────────────────────────────────────────────────
	deploySpec := deployments.Resolve(types.DeploymentTemplateSource{
		Name:               obj.Name + "-deploy",
		Namespace:          ns,
		Image:              obj.Spec.Image,
		Replicas:           obj.Spec.Replicas,
		Port:               obj.Spec.Port,
		ServiceAccountName: obj.Name + "-sa",
	}, obj.Name, reg)
	if err := deployments.Update(ctx, kube, obj, deploySpec); err != nil {
		return domain.Result{}, fmt.Errorf("deployment: %w", err)
	}

	// ── StatefulSet ───────────────────────────────────────────────────────────
	stsSpec := statefulsets.Resolve(types.StatefulSetTemplateSource{
		Name:      obj.Name + "-sts",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Replicas:  obj.Spec.Replicas,
		Port:      obj.Spec.Port,
	}, obj.Name, reg)
	if err := statefulsets.Update(ctx, kube, obj, stsSpec); err != nil {
		return domain.Result{}, fmt.Errorf("statefulset: %w", err)
	}

	// ── ReplicaSet ────────────────────────────────────────────────────────────
	rsSpec := replicasets.Resolve(types.ReplicaSetTemplateSource{
		Name:      obj.Name + "-rs",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Replicas:  obj.Spec.Replicas,
		Port:      obj.Spec.Port,
	}, obj.Name, reg)
	if err := replicasets.Update(ctx, kube, obj, rsSpec); err != nil {
		return domain.Result{}, fmt.Errorf("replicaset: %w", err)
	}

	// ── Pod ───────────────────────────────────────────────────────────────────
	podSpec := pods.Resolve(types.PodTemplateSource{
		Name:      obj.Name + "-pod",
		Namespace: ns,
		Image:     obj.Spec.Image,
		Port:      obj.Spec.Port,
	}, obj.Name, reg)
	if err := pods.Update(ctx, kube, obj, podSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pod: %w", err)
	}

	// ── Job (Create — no Update) ──────────────────────────────────────────────
	jobSpec := jobs.Resolve(types.JobTemplateSource{
		Name:      obj.Name + "-init",
		Namespace: ns,
		Image:     "alpine:3.19",
		Command:   []string{"/bin/sh", "-c", "echo probe-job-ok"},
	}, 3, obj.Name, reg)
	if err := jobs.Create(ctx, kube, obj, jobSpec); err != nil {
		return domain.Result{}, fmt.Errorf("job: %w", err)
	}

	// ── CronJob ───────────────────────────────────────────────────────────────
	cronSpec := cronjobs.Resolve(types.CronJobTemplateSource{
		Name:      obj.Name + "-cron",
		Namespace: ns,
		Schedule:  obj.Spec.Schedule,
		Image:     "alpine:3.19",
		Command:   []string{"/bin/sh", "-c", "echo probe-cron-ok"},
	}, obj.Name, reg)
	if err := cronjobs.Update(ctx, kube, obj, cronSpec); err != nil {
		return domain.Result{}, fmt.Errorf("cronjob: %w", err)
	}

	// ── Service ───────────────────────────────────────────────────────────────
	svcSpec := services.Resolve(types.ServiceTemplateSource{
		Name:       obj.Name + "-svc",
		Namespace:  ns,
		Port:       "80",
		TargetPort: obj.Spec.Port,
	}, obj.Name)
	if err := services.Update(ctx, kube, obj, svcSpec); err != nil {
		return domain.Result{}, fmt.Errorf("service: %w", err)
	}

	// ── Ingress ───────────────────────────────────────────────────────────────
	ingSpec := ingresses.Resolve(types.IngressTemplateSource{
		Name:        obj.Name + "-ing",
		Namespace:   ns,
		Host:        obj.Name + ".probe.local",
		ServiceName: obj.Name + "-svc",
		ServicePort: "80",
	}, obj.Name)
	if err := ingresses.Update(ctx, kube, obj, ingSpec); err != nil {
		return domain.Result{}, fmt.Errorf("ingress: %w", err)
	}

	// ── HPA ───────────────────────────────────────────────────────────────────
	hpaSpec := hpas.Resolve(types.HPATemplateSource{
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
	if err := hpas.Update(ctx, kube, obj, hpaSpec); err != nil {
		return domain.Result{}, fmt.Errorf("hpa: %w", err)
	}

	// ── PDB ───────────────────────────────────────────────────────────────────
	pdbSpec := pdbs.Resolve(types.PDBTemplateSource{
		Name:         obj.Name + "-pdb",
		Namespace:    ns,
		Selector:     map[string]string{"app": obj.Name + "-deploy"},
		MinAvailable: "1",
	}, obj.Name, reg)
	if err := pdbs.Update(ctx, kube, obj, pdbSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pdb: %w", err)
	}

	// ── PV ────────────────────────────────────────────────────────────────────
	pvSpec := pvs.Resolve(types.PVTemplateSource{
		Name:        obj.Name + "-pv",
		Capacity:    obj.Spec.Storage,
		AccessModes: []string{"ReadWriteOnce"},
		HostPath:    "/tmp/inrun-" + obj.Name,
	}, obj.Name)
	if err := pvs.Update(ctx, kube, obj, pvSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pv: %w", err)
	}

	// ── PVC ───────────────────────────────────────────────────────────────────
	pvcSpec := pvcs.Resolve(types.PVCTemplateSource{
		Name:        obj.Name + "-pvc",
		Namespace:   ns,
		Storage:     obj.Spec.Storage,
		AccessModes: []string{"ReadWriteOnce"},
		VolumeName:  obj.Name + "-pv",
	}, obj.Name)
	if err := pvcs.Update(ctx, kube, obj, pvcSpec); err != nil {
		return domain.Result{}, fmt.Errorf("pvc: %w", err)
	}

	return domain.Result{}, nil
}

// onDelete is a no-op: ResourceProbe is cluster-scoped, so owner references work
// for all resources (namespaced in probe-ns and cluster-scoped). GC handles cleanup.
func onDelete(_ context.Context, _ domain.Request, _ *rpv1alpha1.ResourceProbe) (domain.Result, error) {
	return domain.Result{}, nil
}
