# rescope

`AppConfig` moves from a namespaced CRD to a cluster-scoped one. Kubernetes cannot change the scope of an existing CRD, so the kind stays and the group changes: `appconfigs.demo.inrun.dev` (namespaced) → `appconfigs.platform.demo.inrun.dev` (cluster-scoped).

The catch is names. `team-a/payments` and `team-c/payments` are two CRs while namespaced, and one name once cluster-scoped. `migrate.to` never lets one overwrite the other.

## The declaration

```yaml
spec:
  crds:
    appconfig:
      crdFile: ./manifests/crd-appconfig.yaml
      migrate:
        to:
          crdFile: ./manifests/crd-appconfig-cluster.yaml
          carryStatus: true
        # prune: true
    platform-appconfig:
      crdFile: ./manifests/crd-appconfig-cluster.yaml
```

`carryStatus: true` copies each source's status onto its destination. The destination CRD has a status subresource, so status is written through `/status`. Without `carryStatus` the destination starts with no status and its reconciler writes one. Either way the snapshot keeps the source's status, and rollback restores it.

## Walkthrough

### 1. Install the CRDs and four AppConfigs

```bash
inrun create cluster
kubectl apply -f manifests/crd-appconfig.yaml -f manifests/crd-appconfig-cluster.yaml -f manifests/namespaces.yaml
kubectl apply -f manifests/crs.yaml
kubectl get appconfigs.demo.inrun.dev -A
```

```console
NAMESPACE   NAME       IMAGE                  MINREPLICAS   LOGLEVEL   AGE
team-a      payments   myapp/payments:1.0.0   6             info       0s
team-b      search     myapp/search:1.0.0     3             warn       0s
team-c      payments   myapp/payments:1.0.0   2             debug      0s
team-d      ledger     myapp/ledger:1.0.0     4             info       0s
```

### 2. Validate

The scope of each CRD comes from its crdFile:

```bash
inrun validate
```

```console
● appconfig
    kind: AppConfig / group: demo.inrun.dev / version: v1 / plural: appconfigs / scope: Namespaced
    ⚠ warning: CRD: appconfig - AppConfig is cluster-scoped; source namespaces are dropped, and sources with the same name in different namespaces will fail as collisions.
● platform-appconfig
    kind: AppConfig / group: platform.demo.inrun.dev / version: v1 / plural: appconfigs / scope: ClusterScoped
```

### 3. Dry-run catches the collision

```bash
inrun migrate --dry-run
```

```console
◆  AppConfig → AppConfig.platform.demo.inrun.dev  4 CRs
  ✓ team-a/payments  would migrate to AppConfig.platform.demo.inrun.dev
  ✓ team-b/search  would migrate to AppConfig.platform.demo.inrun.dev
  ✗ team-c/payments  destination payments is also the target of team-a/payments in this run
  ✓ team-d/ledger  would migrate to AppConfig.platform.demo.inrun.dev

  ✗ 3/4 would be migrated · 1 failed
```

Nothing exists yet, so the run itself tracks which destination names it has handed out. The first source to claim a name keeps it; the second fails.

### 4. Migrate

```bash
inrun migrate
```

Verify:

```bash
kubectl get appconfigs.platform.demo.inrun.dev \
  -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,TEAM:.metadata.labels.team
```

```console
NAME       PHASE   TEAM
ledger     Ready   team-d
payments   Ready   team-a
search     Ready   team-b
```

The status came across with `carryStatus`, and the source labels with the default `preserveSource`. Each destination's snapshot records where it came from, namespace included:

```bash
kubectl get appconfig.platform.demo.inrun.dev payments \
  -o jsonpath='{.metadata.annotations.inrun\.dev/migrate-kind-snapshot}' | jq -c '{source: .source.metadata, status: .source.status}'
```

```json
{"source":{"name":"payments","namespace":"team-a","labels":{"app":"payments","team":"team-a"}},"status":{"phase":"Ready"}}
```

`team-c/payments` stays a namespaced AppConfig. Re-running reports it again: its name is taken. Rename it (or leave it behind) before the old CRD goes.

### 5. Prune the ones that moved

Uncomment `prune: true`, then narrow the run to the namespaces that moved
cleanly:

```bash
inrun migrate -n team-a,team-b,team-d --yes
```

```console
  ✓ team-a/payments  already migrated; source pruned
  ✓ team-b/search  already migrated; source pruned
  ✓ team-d/ledger  already migrated; source pruned

  ✓ 0/3 migrated · 3 sources pruned
```

Only `team-c/payments` is left in the namespaced CRD.

```bash
kubectl get appconfigs.demo.inrun.dev -A
```

### 6. Roll back

Dry run:

```bash
inrun migrate rollback --dry-run
```

```console
  ✓ team-d/ledger  would roll back to AppConfig; would delete AppConfig.platform.demo.inrun.dev
  ✓ team-a/payments  would roll back to AppConfig; would delete AppConfig.platform.demo.inrun.dev
  ✓ team-b/search  would roll back to AppConfig; would delete AppConfig.platform.demo.inrun.dev

  ✓ 3/3 would be rolled back
```

Rollback for real:

```bash
inrun migrate rollback
kubectl get appconfigs.demo.inrun.dev -A -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,PHASE:.status.phase
```

```console
  ✓ team-d/ledger  rolled back to AppConfig; AppConfig.platform.demo.inrun.dev deleted
  ✓ team-a/payments  rolled back to AppConfig; AppConfig.platform.demo.inrun.dev deleted
  ✓ team-b/search  rolled back to AppConfig; AppConfig.platform.demo.inrun.dev deleted

NS       NAME       PHASE
team-a   payments   Ready
team-b   search     Ready
team-c   payments   Pending
team-d   ledger     Ready
```

Each source is recreated in the namespace the snapshot recorded, with its status, before its destination is deleted.

## The other direction

A cluster-scoped source has no namespace to carry, so moving it to a namespaced CRD needs one:

```yaml
    platform-appconfig:
      crdFile: ./manifests/crd-appconfig-cluster.yaml
      migrate:
        to:
          crdFile: ./manifests/crd-appconfig.yaml
          namespace: platform
```

Without `namespace`, validation fails:

```console
✗ CRD platform-appconfig: migrate.to.namespace is required: the source is cluster-scoped and AppConfig is namespaced
```

## Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

## E2E

Runs the complete CRD rescope lifecycle — the example migrates namespaced `AppConfig` resources from `demo.inrun.dev` to the cluster-scoped `platform.demo.inrun.dev` API group, preserving resource specifications, labels, and status. It verifies migration of resources across namespaces, handles a name collision without overwriting the existing destination, and confirms that source resources remain when pruning is disabled. The example then tests repeated migration, enables pruning to remove successfully migrated sources, verifies that the colliding source remains untouched, and rolls back the migration, restoring the original namespaced resources and their statuses while removing the cluster-scoped destinations.

```
inrun e2e --report
```

`--report` writes results to `report.md` by default. Pass a custom file path to change the output format (e.g. `--report results.json` for JSON).