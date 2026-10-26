# 04-configmap-copy

A namespaced `ConfigMapDistribution`, created in `platform` beside the `app-config` ConfigMap, copies it into each target namespace and keeps the copies in sync.

It is the same job as [03-secret-copy](../03-secret-copy/), with one difference that matters: *the CR is namespaced.*

| | 03-secret-copy | 04-configmap-copy |
| --- | --- | --- |
| CRD scope | Cluster | Namespaced |
| Source namespace | Named in the spec | The CR's own |
| Copies' owner reference | Yes: a cluster-scoped owner can own anything | None: Kubernetes rejects an owner in another namespace |
| Removed on delete by | Kubernetes' garbage collector | Inrun, which finds every child by its `inrun.dev/owner-uid` label, in any namespace |

So a team can share its own config without a cluster-scoped CRD, and the
copies still go when the CR does.

### 1. Simulate

No cluster needed.

```bash
inrun simulate
```

### 2. Run

Applies `manifests/setup.yaml` first: the `platform`, `team-alpha` and
`team-beta` namespaces and the source ConfigMap.

```bash
inrun
```

### 3. Check a copy

In a second terminal. The copy has the owner-uid label and no owner reference.

```bash
kubectl get configmap app-config -n team-alpha -o yaml
```

### 4. Delete the CR

The copies in `team-alpha` and `team-beta` are removed with it.

```bash
kubectl delete configmapdistribution app-config-distribution -n platform
kubectl get configmap app-config -n team-alpha
```

### 5. Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 6. E2E

Runs the same checks in a kind cluster, including the removal on delete.

```bash
inrun e2e
```
