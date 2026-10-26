# Resources

Kubernetes built-ins and custom resources declarable under `onCreate`, `onReconcile`, and `onDelete` in a Blueprint. Each page documents one resource kind's full set of fields — the same schema is reused across all three lifecycle blocks, so a resource's fields don't change depending on which one it's declared under.

## Reference

| Kind | YAML key |
|---|---|
| [Deployment](deployments.md) | `deployments` |
| [ReplicaSet](replicasets.md) | `replicaSets` |
| [Service](services.md) | `services` |
| [Pod](pods.md) | `pods` |
| [Job](jobs.md) | `jobs` |
| [CronJob](cronjobs.md) | `cronJobs` |
| [Secret](secrets.md) | `secrets` |
| [ConfigMap](configmaps.md) | `configMaps` |
| [ServiceAccount](serviceaccounts.md) | `serviceAccounts` |
| [StatefulSet](statefulsets.md) | `statefulSets` |
| [Ingress](ingresses.md) | `ingresses` |
| [PersistentVolume](persistentvolumes.md) | `persistentVolumes` |
| [PersistentVolumeClaim](persistentvolumeclaims.md) | `persistentVolumeClaims` |
| [HorizontalPodAutoscaler](horizontalpodautoscalers.md) | `hpa` |
| [PodDisruptionBudget](poddisruptionbudgets.md) | `pdb` |
| [Namespace](namespaces.md) | `namespaces` |
| [Role](roles.md) | `roles` |
| [RoleBinding](rolebindings.md) | `roleBindings` |
| [ClusterRole](clusterroles.md) | `clusterRoles` |
| [ClusterRoleBinding](clusterrolebindings.md) | `clusterRoleBindings` |
| [LimitRange](limitranges.md) | `limitRanges` |
| [ResourceQuota](resourcequotas.md) | `resourceQuotas` |
| [NetworkPolicy](networkpolicies.md) | `networkPolicies` |
| [Custom Resource](custom.md) | `custom` |
