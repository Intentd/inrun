# 24 — ReplicaSet Notes

ReplicaSet notes navigate owner relationships and the ReplicaSet inventory embedded by enrichment layers. Use them to surface the controlling Deployment name or count historical ReplicaSets for rollback visibility.

---

## Reference

### `replicaSetOwnerName`

Returns `_owner.name` — the name of the controller that owns this ReplicaSet (typically a Deployment). Requires `enrich: [owner]`.

Keywords: replicaset, owner, deployment, name, enriched, string, controller

```yaml
- path: ownerName
  value: "{{ replicaSetOwnerName .children.replicaset }}"
# → "my-app"
```

---

### `replicaSetOwnerKind`

Returns `_owner.kind` — the kind of the controlling resource. Requires `enrich: [owner]`.

Keywords: replicaset, owner, deployment, kind, enriched, string, controller

```yaml
- path: ownerKind
  value: "{{ replicaSetOwnerKind .children.replicaset }}"
# → "Deployment"
```

---

### `deploymentReplicaSetCount`

Returns the number of ReplicaSets owned by the Deployment, including scaled-to-zero ones kept for rollback. Requires `enrich: [replicasets]`.

Keywords: deployment, replicaset, count, rollback, enriched, int, history

```yaml
- path: replicaSetCount
  value: "{{ deploymentReplicaSetCount .children.deployment }}"
# → 2
```

---

### `deploymentReplicaSets`

Returns a list of the names of all ReplicaSets owned by the Deployment, including inactive (scaled‑to‑zero) ones kept for rollback. Requires `enrich: [replicasets]`.

Keywords: deployment, replicaset, names, rollback, enriched, list, history, all

```yaml
- path: replicaSetNames
  value: "{{ deploymentReplicaSets .children.deployment }}"
# → "my-app-7cb76d588c", "my-app-d9c74fd9b"
```

---

### `oldDeploymentReplicaSets`

Returns a list of the names of "old" ReplicaSets owned by the Deployment — i.e., those whose `pod-template-hash` does **not** match the Deployment's current template hash. Requires `enrich: [replicasets]`.

Keywords: deployment, replicaset, old, rollback, enriched, list, inactive, stale

```yaml
- path: oldReplicaSetNames
  value: "{{ oldDeploymentReplicaSets .children.deployment }}"
# → "my-app-7cb76d588c"
```

---

## Quick reference

| Note | Signature | Returns | Enrichment |
|------|-----------|---------|------------|
| `replicaSetOwnerName` | `(obj any)` | `string` | `enrich: [owner]` |
| `replicaSetOwnerKind` | `(obj any)` | `string` | `enrich: [owner]` |
| `deploymentReplicaSetCount` | `(obj any)` | `int` | `enrich: [replicasets]` |
| `deploymentReplicaSets` | `(obj any)` | `[]string` | `enrich: [replicasets]` |
| `oldDeploymentReplicaSets` | `(obj any)` | `[]string` | `enrich: [replicasets]` |

---

**Next →** [Back to README](README.md)
