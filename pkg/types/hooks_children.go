package types

// DeclaredChildKinds returns the set of built-in resource kind keys declared
// anywhere across OnCreate and OnReconcile, keyed to match
// pkg/children.ChildGVRs()'s Key strings exactly.
//
// Built on VisitResources so it stays exhaustive automatically as new
// resource kinds are added to HookTemplates — the only hand-maintained part
// is the type-switch mapping a concrete template type to its key string,
// colocated here next to HookTemplates itself rather than duplicated in a
// distant caller. CustomResourceTemplateSource is intentionally excluded —
// custom resources have dynamic GVRs resolved separately, not through this
// built-in-kind allowlist.
func (b *OperatorBoxConfig) DeclaredChildKinds() map[string]bool {
	if b == nil {
		return nil
	}
	kinds := make(map[string]bool)

	visit := func(ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			switch res.(type) {
			case DeploymentTemplateSource:
				kinds["deployment"] = true
			case ReplicaSetTemplateSource:
				kinds["replicaset"] = true
			case StatefulSetTemplateSource:
				kinds["statefulset"] = true
			case ServiceTemplateSource:
				kinds["service"] = true
			case JobTemplateSource:
				kinds["job"] = true
			case CronJobTemplateSource:
				kinds["cronjob"] = true
			case SecretTemplateSource:
				kinds["secret"] = true
			case ConfigMapTemplateSource:
				kinds["configmap"] = true
			case ServiceAccountTemplateSource:
				kinds["serviceaccount"] = true
			case NamespaceTemplateSource:
				kinds["namespace"] = true
			case IngressTemplateSource:
				kinds["ingress"] = true
			case PVTemplateSource:
				kinds["persistentvolume"] = true
			case PVCTemplateSource:
				kinds["persistentvolumeclaim"] = true
			case RoleTemplateSource:
				kinds["role"] = true
			case RoleBindingTemplateSource:
				kinds["rolebinding"] = true
			case ClusterRoleTemplateSource:
				kinds["clusterrole"] = true
			case ClusterRoleBindingTemplateSource:
				kinds["clusterrolebinding"] = true
			case HPATemplateSource:
				kinds["horizontalpodautoscaler"] = true
			case PDBTemplateSource:
				kinds["poddisruptionbudget"] = true
			case LimitRangeTemplateSource:
				kinds["limitrange"] = true
			case ResourceQuotaTemplateSource:
				kinds["resourcequota"] = true
			case NetworkPolicyTemplateSource:
				kinds["networkpolicy"] = true
			}
		})
	}

	visit(b.EffectiveOnCreate())
	visit(b.EffectiveOnReconcile())

	if len(kinds) == 0 {
		return nil
	}
	return kinds
}
