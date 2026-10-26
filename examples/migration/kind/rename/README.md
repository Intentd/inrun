# rename

`AppConfig` CRs move to a new CRD, `NewConfig`. Intent-based callers don't change, the reconciler doesn't change, and Kubernetes stays satisfied at every step.

## Why this works

The CRD is storage, not the API. Three things talk to it, and none of them needs to depend on its shape:

- **Callers** send intents (`image`, `replicas`, `logLevel`) through the gateway. `serve.fields` maps them to spec paths, so a path or kind change is invisible to intent-based callers.
- **The reconciler** gets the arguments the coordinator prepared and returns what should be true. It does not need to read or write the CR's storage shape directly.
- **Kubernetes** checks that each custom resource matches an installed CRD.

So the platform can replace the CRD. `migrate.to` makes that a declared, reversible step, like an Alembic migration beneath SQL declarations: it moves stored CRs, keeps a snapshot on each destination, and supports rollback.

There is one important API boundary: **full-CR callers explicitly submit an API version and kind.** The gateway must validate those against the configured destination during migration. It must not silently rewrite a submitted `AppConfig` into a `NewConfig`.

Anything that still names `spec.*` directly — blueprint templates and conditions, printer columns, and CRs written with `kubectl` — is what the CRD-file checks below help guard.

## The declaration

```yaml
spec:
  crds:
    appconfig:
      crdFile: ./manifests/crd-appconfig.yaml
      migrate:
        to:
          crdFile: ./manifests/crd-newconfig.yaml # fills apiVersion, kind, plural
          metadata:
            labels:
              migrated-from: appconfig            # added on every destination
        selection:
          selector: tier=web                      # filtered by the API server
          when:
            - field: status.phase                 # evaluated per CR
              equals: Ready
        # prune: true
    newconfig:
      crdFile: ./manifests/crd-newconfig.yaml
      serve: { target: my-new-api-server, include: ./serve/appconfig.yaml, ... }
```

Each selected source becomes a `NewConfig` with the same name, namespace, and spec. Source labels and annotations are carried over (tooling keys such as `kubectl.kubernetes.io/*` and `inrun.dev/*` are dropped) unless `metadata.preserveSource: false`; declared destination keys win.

Status is not carried in this example. The destination's reconciler writes it. Rollback recreates a pruned source without status for the same reason, so a `when` condition on `status.*` will not match a restored source until its reconciler has run.

`selection` works at either level: a `migrate:` block without `to` scopes the field-level `migrate.from` migrations of that CRD instead.

## Checked at validate time

With both `crdFile` declarations present, `inrun validate` checks the move before anything touches the cluster.

- **The destination schema holds every source field.** Drop `scaling` from `crd-newconfig.yaml` and validation fails:

  ```console
  ✗ CRD appconfig: NewConfig cannot hold every source field:
    spec.scaling.min: missing in destination
  ```

- **Scopes.** Changing scope is covered in [rescope](../rescope/README.md).

- Declared `apiVersion`/`kind`/`plural` must match the destination CRD file, the destination cannot be the source itself, and selection conditions must be well-formed.

## Walkthrough

### 1. Install both CRDs and the AppConfigs

```bash
inrun create cluster
kubectl apply -f manifests/crd-appconfig.yaml -f manifests/crd-newconfig.yaml -f manifests/namespaces.yaml
kubectl apply -f manifests/crs-appconfig.yaml
kubectl get appconfigs -A -L tier
```

```console
NAMESPACE     NAME        IMAGE             MINREPLICAS   LOGLEVEL   AGE   TIER
platform-01   api-server  myapp/api:2.2.0   6             warn       2s    web
platform-02   api-server  myapp/api:2.2.0   6             warn       2s    web
platform-03   api-server  myapp/api:2.2.0   6             warn       2s    web
platform-04   api-server  myapp/api:2.2.0   6             warn       2s    batch
...
platform-10   api-server  myapp/api:2.2.0   6             warn       1s
```

There are ten source CRs across `web`, `batch`, and `internal` tiers, plus one with no tier. The `web` and `internal` resources are `Ready`; the others are `Pending`.

### 2. Start the gateway before migration

Start the gateway in **Terminal 1**, without `--migrate`:

```bash
DEV_TOKEN=dev-token inrun gate run
```

Leave it running throughout the rest of the walkthrough. The migration will be invoked separately from Terminal 2.

### 3. Create a resource through the intent API

In **Terminal 2**, send an intent to the gateway:

```bash
curl -s -X POST localhost:8080/api/v1/apply \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{"target":"my-api-server","image":"myapp/api:3.0.0","replicas":4,"logLevel":"info"}' | jq
```

Example response:

```json
{
  "accepted": true,
  "name": "api-server-gateway",
  "namespace": "default",
  "kind": "NewConfig",
  "apiVersion": "demo.inrun.dev/v1",
  "pollUrl": "/api/v1/resources/NewConfig/default/api-server-gateway"
}
```

Check the destination resource:

```bash
kubectl get newconfigs -A
```

The gateway creates a `NewConfig` even though what is defined as served in the [Blueprint](./blueprint.yaml) is `AppConfig` and its old resources have not yet been migrated.

This is the key behavior: intent-based callers do not have to wait for the stored resources to be migrated before using the destination API.

### 4. The old full-CR request is rejected

A caller submits the old `AppConfig` through the gateway - [`manifests/cr-for-gateway.yaml`](./manifests/cr-for-gateway.yaml):

```bash
curl -s -X POST localhost:8080/api/v1/apply \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/yaml" \
  --data-binary @manifests/cr-for-gateway.yaml | jq
```

The gateway rejects the request because the submitted GVK is `AppConfig`, while the configured migration destination is `NewConfig`. It does not silently rewrite a full CR's GVK.

```json
{
  "error": "source CRD under migration",
  "message": "kind \"AppConfig\" is being migrated; submit the destination kind \"NewConfig\" with apiVersion \"demo.inrun.dev/v1\""
}
```

Intent-based callers continue using the same flat request. Full-CR callers explicitly submit the resource's GVK, so requests using the old GVK must be updated to use the destination GVK.

This is intentional: intent-based requests use the gateway's declared field mapping and can continue through the rename, while full-CR requests must explicitly use the accepted GVK. The rejection happens during GVK validation, before the gateway attempts to apply the resource.

### 5. Validate and dry-run the migration

With the gateway still running, run:

```bash
inrun validate
inrun migrate --dry-run
```

```console
◆  AppConfig → NewConfig  3 CRs
   scope: selector tier=web
   ✓ platform-01/api-server  would migrate to NewConfig
   ✓ platform-02/api-server  would migrate to NewConfig
   ✓ platform-03/api-server  would migrate to NewConfig

   ✓ 3/3 would be migrated
```

The `scope:` line shows the selection used by the migration. The dry-run applies each destination with server-side dry-run, so `--verbose` shows the spec the API server would store, including defaults:

```bash
inrun migrate --dry-run --verbose --limit 1
```

The gateway is not restarted and does not need to be stopped for this step.

### 6. Narrow from the CLI — never widen

CLI flags combine with `migrate.selection`: namespaces intersect, selectors AND, and names must agree.

```bash
inrun migrate --dry-run -n platform-02,platform-07
```

```console
◆  AppConfig → NewConfig  1 CR
   scope: namespaces platform-02,platform-07 · selector tier=web
   ✓ platform-02/api-server  would migrate to NewConfig
```

`--selector tier=internal` combines with the declared `tier=web` selector, producing `tier=internal,tier=web` and selecting nothing. A namespace outside a declared `selection.namespaces` is an error.

### 7. Test the `when` condition

Change one selected resource so it no longer satisfies the status condition:

```bash
kubectl patch appconfig api-server -n platform-03 --type merge \
  -p '{"status":{"phase":"Pending"}}'
inrun migrate --dry-run
```

```console
   ✓ platform-01/api-server  would migrate to NewConfig
   ✓ platform-02/api-server  would migrate to NewConfig
   – platform-03/api-server  (not selected: migrate.selection when/or)

   ✓ 2/3 would be migrated · 1 not selected
```

The `when` condition is evaluated for each source CR. The resource in `platform-03` is excluded while its phase is `Pending`.

### 8. Run the migration while the gateway stays up

Run the forward migration from Terminal 2:

```bash
inrun migrate
kubectl get newconfigs -A -L migrated-from,tier
```

```console
NAMESPACE     NAME                 IMAGE             MINREPLICAS   LOGLEVEL   AGE     MIGRATED-FROM   TIER
default       api-server-gateway   myapp/api:3.0.0   4             info       6m34s                   
platform-01   api-server           myapp/api:2.2.0   6             warn       0s      appconfig       web
platform-02   api-server           myapp/api:2.2.0   6             warn       0s      appconfig       web
```

The source resources remain because pruning is off. Each migrated destination carries a snapshot of the source:

```bash
kubectl get newconfig api-server -n platform-01 \
  -o jsonpath='{.metadata.annotations.inrun\.dev/migrate-kind-snapshot}' | jq
```

```json
{
  "source": {
    "apiVersion": "demo.inrun.dev/v1",
    "kind": "AppConfig",
    "metadata": {
      "name": "api-server",
      "namespace": "platform-01",
      "labels": {
        "app": "demo",
        "tier": "web"
      }
    },
    "spec": {
      "image": "myapp/api:2.2.0",
      "observability": {
        "logging": {
          "level": "warn"
        }
      },
      "scaling": {
        "min": 6
      }
    }
  },
  "destination": {
    "apiVersion": "demo.inrun.dev/v1",
    "kind": "NewConfig",
    "metadata": {
      "name": "api-server",
      "namespace": "platform-01"
    }
  },
  "pruned": false
}
```

Run the migration again to verify idempotency:

```bash
inrun migrate
```

```console
   – platform-01/api-server  (already migrated)
   – platform-02/api-server  (already migrated)
   – platform-03/api-server  (not selected: migrate.selection when/or)

   ✓ 0/3 migrated · 2 already done · 1 not selected
```

**Important:** if the gateway has already created or updated a `NewConfig` with the same name and namespace as a source selected for migration, the migration engine must have a defined conflict policy for that existing destination. The fact that the two resources have different kinds avoids a Kubernetes name collision, but it does not resolve a logical conflict between the gateway-written destination and the source data. Confirm that behavior before using overlapping source and destination identities in production.

### 9. Send the same intent after migration

With the gateway process still running, send the same intent request again:

```bash
curl -s -X POST localhost:8080/api/v1/apply \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{"target":"my-api-server","image":"myapp/api:3.0.0","replicas":4,"logLevel":"info"}'  | jq
```

The caller sends exactly the same intent as before migration. The gateway continues to target `NewConfig`; there is no gateway restart and no caller-side change.

The full-CR request using `kind: AppConfig` remains rejected. Callers using full-CR mode must change the submitted GVK to the destination GVK to be accepted.

### 10. Prune the sources

Set `platform-03` back to `Ready` and uncomment `prune: true` in [`blueprint.yaml`](./blueprint.yaml):

```bash
kubectl patch appconfig api-server -n platform-03 --type merge \
  -p '{"status":{"phase":"Ready"}}'
inrun migrate
```

The command counts the sources it would delete and asks for confirmation:

```console
⚠ migrate.prune is on. These source CRs are deleted after their destination is written:
    AppConfig → NewConfig  3 sources

Delete 3 source CRs after migrating? [y/N]:
```

Without a terminal, the command stops and asks for `--yes`; `--dry-run` never asks.

```bash
inrun migrate --yes
```

```console
   ✓ platform-01/api-server  already migrated; source pruned
   ✓ platform-02/api-server  already migrated; source pruned
   ✓ platform-03/api-server  migrated to NewConfig; source pruned

   ✓ 1/3 migrated · 3 sources pruned
```

Confirm:

```bash
kubectl get appconfigs -A -L migrated-from,tier
kubectl get newconfigs -A -L migrated-from,tier
```

```console
kubectl get newconfigs -A -L migrated-from,tier
NAMESPACE     NAME                 IMAGE             MINREPLICAS   LOGLEVEL   AGE    MIGRATED-FROM   TIER
default       api-server-gateway   myapp/api:3.0.0   4             info       4m6s                   
platform-01   api-server           myapp/api:2.2.0   6             warn       70s    appconfig       web
platform-02   api-server           myapp/api:2.2.0   6             warn       69s    appconfig       web
platform-03   api-server           myapp/api:2.2.0   6             warn       69s    appconfig       web
rename$ kubectl get appconfigs -A -L migrated-from,tier
NAMESPACE     NAME         IMAGE             MINREPLICAS   LOGLEVEL   AGE    MIGRATED-FROM   TIER
platform-04   api-server   myapp/api:2.2.0   6             warn       5m4s                   batch
platform-05   api-server   myapp/api:2.2.0   6             warn       5m4s                   batch
platform-06   api-server   myapp/api:2.2.0   6             warn       5m4s                   batch
platform-07   api-server   myapp/api:2.2.0   6             warn       5m4s                   internal
platform-08   api-server   myapp/api:2.2.0   6             warn       5m4s                   internal
platform-09   api-server   myapp/api:2.2.0   6             warn       5m4s                   internal
platform-10   api-server   myapp/api:2.2.0   6             warn       5m4s                   
rename$ 
```

A source is pruned only after its destination holds every source field. The delete is conditional on the source's `resourceVersion`: if anything writes to the source after it was read, pruning is refused and the CR is reported, so a concurrent source write is never silently lost.

Pruning is a plain delete, so Kubernetes garbage-collects anything the source owns. Have the destination's reconciler take over those children first.

### 11. Roll back

```bash
inrun migrate rollback --dry-run
inrun migrate rollback --name api-server -n platform-01
```

```console
◆  AppConfig → NewConfig  1 CR
   scope: namespaces platform-01 · name api-server · selector tier=web
   ✓ platform-01/api-server  rolled back to AppConfig; NewConfig deleted
```

Rollback reads destination snapshots that identify `AppConfig` as the source and matches the selection against the recorded source. It recreates the source first and deletes the destination only after recreation succeeds. A destination edited after migration is reported and left in place.

### 12. Clean

Once the move is final, drop the snapshots:

```bash
inrun migrate clean --dry-run
inrun migrate clean
```

After `clean`, rollback no longer has the snapshots it needs to restore the source resources.

## Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

E2E

Runs the complete CRD rename lifecycle — the gateway starts without migration enabled and remains available throughout, intent requests create `NewConfig` resources before migration, and full-CR requests using the old `AppConfig` GVK are rejected. The example then migrates selected `AppConfig` resources to `NewConfig`, verifies selector and condition-based filtering, tests pruning, rolls back a destination, and cleans up the remaining snapshots. Gateway intent continues working before migration, after migration, and after rollback without restarting the gateway.

```bash
inrun e2e --report
```

`--report` writes results to `report.md` by default. Pass a custom file path to change the output format (e.g. `--report results.json` for JSON).