package children

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ReadChildren reads all child resources declared in the Blueprint's onCreate
// templates and returns a structured map for use in status field expressions.
//
// The returned map is injected into the template resolver under the "children"
// key. Status field expressions can then reference child resource state:
//
//	# Singular — the first/only resource of this type
//	{{ .children.deployment.status.readyReplicas }}
//	{{ .children.service.status.loadBalancer.ingress }}
//
//	# Plural — all resources of this type, by name
//	{{ (index .children.deployments "my-site-api").status.readyReplicas }}
//
// ReadChildren is called after runTemplates — child resources exist
// at this point. Missing status fields resolve to "" (missingkey=zero),
// which is correct eventual consistency behaviour for newly created resources.
//
// API cost: one GET per child resource type × count.
// For the common case (one Deployment + one Service), this is two GETs.
//
// This function never returns an error that should fail the reconcile.
// Read failures are logged and the child is omitted from the map.
// Status patching proceeds with whatever children were successfully read.
func ReadChildren(
	ctx context.Context,
	kube kubeclient.Interface,
	obj domain.Object,
	resolver *template.Resolver,
	crd types.CRDEntry,
) map[string]interface{} {
	children := map[string]interface{}{}

	// Two-phase enrichment gate evaluation.
	//
	// Phase 1: run only unconditional targets. Resources are read and their
	// unconditional enrichments (e.g. pods) are applied, populating .children.*.
	//
	// Phase 2: after reads complete, re-evaluate conditional targets with the
	// now-populated children map. Gates like {{ allReplicasReady .children.deployment }}
	// depend on child data that does not exist at function entry — evaluating them
	// upfront against the empty resolver always fails.
	//
	// applySecondPhaseEnrichments (below) re-runs enrichment functions for any
	// newly-active conditional targets. Each enricher is a no-op when its target
	// is not in crd.Enrich, so only the newly-active ones do real API work.
	originalEnrich := crd.Enrich
	if !crd.EnrichAll {
		crd.Enrich = crd.UnconditionalEnrichTargets()
	}

	// Collect all template sources across onCreate and onReconcile.
	// We only read resources that are declared — not all resources in the namespace.
	templates := mergeTemplates(crd.Box())

	// ── Deployments ───────────────────────────────────────────────────────
	if len(templates.Deployments) > 0 {
		dNames := deploymentNames(resolver, templates.Deployments)
		m := readResourceGroup(ctx, kube, obj, resolver, DeploymentGVR, dNames)
		// Deployments do not directly own pods — their ReplicaSets do.
		// Filtering by ownerKind=ReplicaSet excludes Job pods that share the
		// same inrun-owner label but have a different immediate controller.
		enrichGroupWithPods(ctx, kube, m, crd, "ReplicaSet")
		enrichGroupWithReplicaSets(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Deployment")
		children["deployments"] = m
		children["deployment"] = firstValue(m)

		// When replicasets enrichment is active but no RSes are declared in onCreate,
		// synthesize .children.replicaset from the active RS embedded in _replicaSets.
		// This makes replicaSetOwnerName and replicaSetOwnerKind work on .children.replicaset.
		if crd.ShouldEnrich("replicasets") && len(templates.ReplicaSets) == 0 {
			if rsGroup := activeReplicaSetGroup(m); len(rsGroup) > 0 {
				children["replicasets"] = rsGroup
				children["replicaset"] = firstValue(rsGroup)
			}
		}
	}

	// ── StatefulSets ───────────────────────────────────────────────────────
	if len(templates.StatefulSets) > 0 {
		dNames := statefulSetNames(resolver, templates.StatefulSets)
		m := readResourceGroup(ctx, kube, obj, resolver, StatefulSetGVR, dNames)
		enrichGroupWithPods(ctx, kube, m, crd, "StatefulSet")
		enrichGroupWithStatefulSetPVCs(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "StatefulSet")
		children["statefulsets"] = m
		children["statefulset"] = firstValue(m)
	}

	// ── ReplicaSets ───────────────────────────────────────────────────────
	if len(templates.ReplicaSets) > 0 {
		dNames := replicaSetNames(resolver, templates.ReplicaSets)
		m := readResourceGroup(ctx, kube, obj, resolver, ReplicaSetGVR, dNames)
		enrichGroupWithPods(ctx, kube, m, crd, "ReplicaSet")
		enrichGroupWithWarnings(ctx, kube, m, crd, "ReplicaSet")
		children["replicasets"] = m
		children["replicaset"] = firstValue(m)
	}

	// ── CustomResources ───────────────────────────────────────────────────────
	// Each entry has its own APIVersion/Kind, so GVR is resolved per entry via
	// RESTMapper rather than using a single shared GVR.
	if len(templates.CustomResource) > 0 {
		m := readCustomResourceGroup(ctx, kube, obj, resolver, templates.CustomResource)
		enrichGroupWithWarnings(ctx, kube, m, crd, "")
		children["customs"] = m
		children["custom"] = firstValue(m)
	}

	// ── Services ──────────────────────────────────────────────────────────
	if len(templates.Services) > 0 {
		svcNames := serviceNames(resolver, templates.Services)
		m := readResourceGroup(ctx, kube, obj, resolver, ServiceGVR, svcNames)
		enrichGroupWithEndpoints(ctx, kube, m, crd)
		enrichGroupWithBackingPods(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Service")
		children["services"] = m
		children["service"] = firstValue(m)

		// Auto-fetch the EndpointSlice for each declared Service.
		esMap := readEndpointSlicesForServices(ctx, kube, obj, svcNames)
		if len(esMap) > 0 {
			children["endpointslices"] = esMap
			children["endpointslice"] = firstValue(esMap)
		}
	}

	// ── Secrets ───────────────────────────────────────────────────────────
	if len(templates.Secrets) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, SecretGVR,
			secretNames(resolver, templates.Secrets))
		enrichGroupWithWarnings(ctx, kube, m, crd, "Secret")
		children["secrets"] = m
		children["secret"] = firstValue(m)
	}

	// ── ConfigMaps ────────────────────────────────────────────────────────
	if len(templates.ConfigMaps) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, ConfigMapGVR,
			configMapNames(resolver, templates.ConfigMaps))
		enrichGroupWithWarnings(ctx, kube, m, crd, "ConfigMap")
		children["configmaps"] = m
		children["configmap"] = firstValue(m)
	}

	// ── Jobs ──────────────────────────────────────────────────────────────
	if len(templates.Jobs) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, JobGVR,
			jobNames(resolver, templates.Jobs))
		enrichGroupWithPods(ctx, kube, m, crd, "Job")
		enrichGroupWithWarnings(ctx, kube, m, crd, "Job")
		children["jobs"] = m
		children["job"] = firstValue(m)
	}

	// ── CronJobs ──────────────────────────────────────────────────────────
	if len(templates.CronJobs) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, CronJobGVR,
			cronJobNames(resolver, templates.CronJobs))
		enrichGroupWithCronJobChildren(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "CronJob")
		children["cronjobs"] = m
		children["cronjob"] = firstValue(m)
	}

	// ── Pods ──────────────────────────────────────────────────────────────
	if len(templates.Pods) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, PodGVR,
			podNames(resolver, templates.Pods))
		enrichGroupWithNode(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Pod")
		children["pods"] = m
		children["pod"] = firstValue(m)
	}

	// ── ServiceAccounts ───────────────────────────────────────────────────
	if len(templates.ServiceAccounts) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, ServiceAccountGVR,
			serviceAccountNames(resolver, templates.ServiceAccounts))
		enrichGroupWithWarnings(ctx, kube, m, crd, "ServiceAccount")
		children["serviceaccounts"] = m
		children["serviceaccount"] = firstValue(m)
	}

	// ── Namespaces ───────────────────────────────────────────────────
	if len(templates.Namespaces) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, NamespaceGVR,
			namespaceNames(resolver, templates.Namespaces))
		enrichGroupWithWarnings(ctx, kube, m, crd, "Namespace")
		children["namespaces"] = m
		children["namespace"] = firstValue(m)
	}

	// ── Ingresses ────────────────────────────────────────────────────────
	if len(templates.Ingresses) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, IngressGVR,
			ingressNames(resolver, templates.Ingresses))
		enrichGroupWithIngressData(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Ingress")
		children["ingresses"] = m
		children["ingress"] = firstValue(m)
	}

	// ── HorizontalPodAutoscalers ─────────────────────────────────────────
	if len(templates.HorizontalPodAutoscalers) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, HorizontalPodAutoscalerGVR,
			hpaNames(resolver, templates.HorizontalPodAutoscalers))
		enrichGroupWithWarnings(ctx, kube, m, crd, "HorizontalPodAutoscaler")
		children["hpas"] = m
		children["hpa"] = firstValue(m)
	}

	// ── PersistentVolumeClaims ────────────────────────────────────────────
	if len(templates.PersistentVolumeClaims) > 0 {
		pvcNms := pvcNames(resolver, templates.PersistentVolumeClaims)
		m := readResourceGroup(ctx, kube, obj, resolver, PersistentVolumeClaimGVR, pvcNms)
		enrichGroupWithPV(ctx, kube, m, crd)
		enrichGroupWithStorageClass(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "PersistentVolumeClaim")
		children["persistentvolumeclaims"] = m
		children["pvc"] = firstValue(m)
	}

	// ── PersistentVolumes ─────────────────────────────────────────────────
	// PVs are cluster-scoped — readResourceGroup skips namespace when empty.
	if len(templates.PersistentVolumes) > 0 {
		pvNms := pvNames(resolver, templates.PersistentVolumes)
		m := readResourceGroup(ctx, kube, obj, resolver, PersistentVolumeGVR, pvNms)
		enrichGroupWithPVC(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "PersistentVolume")
		children["persistentvolumes"] = m
		children["pv"] = firstValue(m)
	}

	// ── NetworkPolicies ───────────────────────────────────────────────────
	if len(templates.NetworkPolicies) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, NetworkPolicyGVR,
			networkPolicyNames(resolver, templates.NetworkPolicies))
		enrichGroupWithWarnings(ctx, kube, m, crd, "NetworkPolicy")
		children["networkpolicies"] = m
		children["networkpolicy"] = firstValue(m)
	}

	// ── ResourceQuotas ────────────────────────────────────────────────────
	if len(templates.ResourceQuotas) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, ResourceQuotaGVR,
			resourceQuotaNames(resolver, templates.ResourceQuotas))
		enrichGroupWithWarnings(ctx, kube, m, crd, "ResourceQuota")
		children["resourcequotas"] = m
		children["resourcequota"] = firstValue(m)
	}

	// ── LimitRanges ───────────────────────────────────────────────────────
	if len(templates.LimitRanges) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, LimitRangeGVR,
			limitRangeNames(resolver, templates.LimitRanges))
		enrichGroupWithWarnings(ctx, kube, m, crd, "LimitRange")
		children["limitranges"] = m
		children["limitrange"] = firstValue(m)
	}

	// ── ClusterRoles ──────────────────────────────────────────────────────
	// Cluster-scoped — readResourceGroup skips namespace when empty.
	if len(templates.ClusterRoles) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, ClusterRoleGVR,
			clusterRoleNames(resolver, templates.ClusterRoles))
		enrichGroupWithWarnings(ctx, kube, m, crd, "ClusterRole")
		children["clusterroles"] = m
		children["clusterrole"] = firstValue(m)
	}

	// ── ClusterRoleBindings ───────────────────────────────────────────────
	// Cluster-scoped — readResourceGroup skips namespace when empty.
	if len(templates.ClusterRoleBindings) > 0 {
		m := readResourceGroup(ctx, kube, obj, resolver, ClusterRoleBindingGVR,
			clusterRoleBindingNames(resolver, templates.ClusterRoleBindings))
		enrichGroupWithWarnings(ctx, kube, m, crd, "ClusterRoleBinding")
		children["clusterrolebindings"] = m
		children["clusterrolebinding"] = firstValue(m)
	}

	// ── Phase 2: conditional enrichment targets ───────────────────────────
	// Now that children are populated, re-evaluate conditional gates.
	//
	// resolver.WithChildren injects children into both r.data AND the
	// TemplateEvaluator closure — both must see .children.* for template
	// expressions like {{ allReplicasReady .children.deployment }} to work.
	// Building data2 manually and reusing the original evaluator does not work:
	// the evaluator is a closure over r.data and ignores data2 entirely.
	if !crd.EnrichAll && len(originalEnrich) > len(crd.Enrich) {
		r2 := resolver.WithChildren(children)
		crd.Enrich = originalEnrich
		active := crd.ConditionalActiveEnrichTargets(r2.Data(), r2.TemplateEvaluator())
		if len(active) > 0 {
			crd.Enrich = active
			applySecondPhaseEnrichments(ctx, kube, children, crd)
		}
	}

	return children
}

// applySecondPhaseEnrichments re-runs enrichment functions against already-read
// resource groups. Called after conditional targets are re-evaluated with children
// data available. Each enricher checks enrichmentEnabled internally — only targets
// present in crd.Enrich do real API work; all others return immediately.
func applySecondPhaseEnrichments(
	ctx context.Context,
	kube kubeclient.Interface,
	children map[string]interface{},
	crd types.CRDEntry,
) {
	if m, ok := children["deployments"].(map[string]interface{}); ok {
		enrichGroupWithPods(ctx, kube, m, crd, "ReplicaSet")
		enrichGroupWithReplicaSets(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Deployment")
	}
	if m, ok := children["statefulsets"].(map[string]interface{}); ok {
		enrichGroupWithPods(ctx, kube, m, crd, "StatefulSet")
		enrichGroupWithStatefulSetPVCs(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "StatefulSet")
	}
	if m, ok := children["replicasets"].(map[string]interface{}); ok {
		enrichGroupWithPods(ctx, kube, m, crd, "ReplicaSet")
		enrichGroupWithWarnings(ctx, kube, m, crd, "ReplicaSet")
	}
	if m, ok := children["customs"].(map[string]interface{}); ok {
		enrichGroupWithWarnings(ctx, kube, m, crd, "")
	}
	if m, ok := children["services"].(map[string]interface{}); ok {
		enrichGroupWithEndpoints(ctx, kube, m, crd)
		enrichGroupWithBackingPods(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Service")
	}
	if m, ok := children["secrets"].(map[string]interface{}); ok {
		enrichGroupWithWarnings(ctx, kube, m, crd, "Secret")
	}
	if m, ok := children["configmaps"].(map[string]interface{}); ok {
		enrichGroupWithWarnings(ctx, kube, m, crd, "ConfigMap")
	}
	if m, ok := children["jobs"].(map[string]interface{}); ok {
		enrichGroupWithPods(ctx, kube, m, crd, "Job")
		enrichGroupWithWarnings(ctx, kube, m, crd, "Job")
	}
	if m, ok := children["cronjobs"].(map[string]interface{}); ok {
		enrichGroupWithCronJobChildren(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "CronJob")
	}
	if m, ok := children["pods"].(map[string]interface{}); ok {
		enrichGroupWithNode(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Pod")
	}
	if m, ok := children["serviceaccounts"].(map[string]interface{}); ok {
		enrichGroupWithWarnings(ctx, kube, m, crd, "ServiceAccount")
	}
	if m, ok := children["namespaces"].(map[string]interface{}); ok {
		enrichGroupWithWarnings(ctx, kube, m, crd, "Namespace")
	}
	if m, ok := children["ingresses"].(map[string]interface{}); ok {
		enrichGroupWithIngressData(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "Ingress")
	}
	if m, ok := children["hpas"].(map[string]interface{}); ok {
		enrichGroupWithWarnings(ctx, kube, m, crd, "HorizontalPodAutoscaler")
	}
	if m, ok := children["persistentvolumeclaims"].(map[string]interface{}); ok {
		enrichGroupWithPV(ctx, kube, m, crd)
		enrichGroupWithStorageClass(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "PersistentVolumeClaim")
	}
	if m, ok := children["persistentvolumes"].(map[string]interface{}); ok {
		enrichGroupWithPVC(ctx, kube, m, crd)
		enrichGroupWithWarnings(ctx, kube, m, crd, "PersistentVolume")
	}
}
