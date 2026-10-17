# ReplicaSet Notes

ReplicaSet notes navigate owner relationships and the ReplicaSet inventory embedded by enrichment layers. Use them to surface the controlling Deployment name or count historical ReplicaSets for rollback visibility.

---

## Reference

| Note | Description |
|------|-------------|
| `replicaSetOwnerName` | Returns `_owner. |
| `replicaSetOwnerKind` | Returns `_owner. |
| `deploymentReplicaSetCount` | Returns the number of ReplicaSets owned by the Deployment, including scaled-to-zero ones kept for rollback. |
| `deploymentReplicaSets` | Returns a list of the names of all ReplicaSets owned by the Deployment, including inactive (scaled‑to‑zero) ones kept for rollback. |
| `oldDeploymentReplicaSets` | Returns a list of the names of "old" ReplicaSets owned by the Deployment — i. |

## Examples

```yaml
# replicaSetOwnerName
- path: ownerName
  value: "{{ replicaSetOwnerName .children.replicaset }}"
# → "my-app"

# replicaSetOwnerKind
- path: ownerKind
  value: "{{ replicaSetOwnerKind .children.replicaset }}"
# → "Deployment"

# deploymentReplicaSetCount
- path: replicaSetCount
  value: "{{ deploymentReplicaSetCount .children.deployment }}"
# → 2

# deploymentReplicaSets
- path: replicaSetNames
  value: "{{ deploymentReplicaSets .children.deployment }}"
# → "my-app-7cb76d588c", "my-app-d9c74fd9b"

# oldDeploymentReplicaSets
- path: oldReplicaSetNames
  value: "{{ oldDeploymentReplicaSets .children.deployment }}"
# → "my-app-7cb76d588c"
```
