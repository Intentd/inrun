# rename

`AppConfig` CRs move to a new CRD, `NewConfig`. Callers don't change, the
reconciler doesn't change, and Kubernetes stays satisfied at every step.

## Why this works

The CRD is storage, not the API. Three things talk to it, and none of them
depends on its shape:

- **Callers** send intents (`image`, `replicas`, `logLevel`) through the
  gateway. `serve.fields` maps them to spec paths, so a path or kind change is
  invisible to them.
- **The reconciler** gets the arguments the coordinator prepared and returns
  what should be true. It does not read or write the CR's storage shape.
- **Kubernetes** only checks that each CR matches the CRD that is installed.

So the platform can replace the CRD. `migrate.to` makes that a declared,
reversible step, the way an Alembic migration sits under SQL declarations:
it moves the stored CRs, keeps a snapshot on each destination, and can roll
back.

What still names `spec.*` directly — blueprint templates and conditions,
printer columns, CRs written with kubectl — is what the crdFile checks below
guard.

## The declaration

```yaml
spec:
  crds:
    appconfig:
      crdFile: ./manifests/crd-appconfig.yaml
      migrate:
        to:
          crdFile: ./manifests/crd-newconfig.yaml   # fills apiVersion, kind, plural
          metadata:
            labels:
              migrated-from: appconfig              # added on every destination
        selection:
          selector: tier=web                        # filtered by the API server
          when:
            - field: status.phase                   # evaluated per CR
              equals: Ready
        # prune: true
    newconfig:
      crdFile: ./manifests/crd-newconfig.yaml
      serve: { target: my-new-api-server, include: ./serve/appconfig.yaml, ... }
```

Each selected source becomes a `NewConfig` with the same name, namespace and
spec. Source labels and annotations are carried (tooling keys such as
`kubectl.kubernetes.io/*` and `inrun.dev/*` dropped) unless
`metadata.preserveSource: false`; declared keys win.

Status is not carried. The destination's reconciler writes it. Rollback
recreates a pruned source without status for the same reason, so a `when` on
`status.*` will not match a restored source until its reconciler has run.

`selection` works at either level: a `migrate:` block without `to` scopes the
field-level `migrate.from` migrations of that CRD instead.

## Checked at validate time

With both crdFiles declared, `inrun validate` checks the move before anything
touches the cluster:

- **The destination schema holds every source field.** Drop `scaling` from
  `crd-newconfig.yaml` and validation fails:

  ```console
  ✗ CRD appconfig: NewConfig cannot hold every source field:
    spec.scaling.min: missing in destination
  ```

- **Scopes.** Changing scope is covered in [rescope](../rescope/README.md).

- Declared `apiVersion`/`kind`/`plural` must match the crdFile, the
  destination cannot be the source itself, and selection conditions must be
  well-formed.

## Walkthrough

### 1. Install both CRDs and the AppConfigs

```bash
inrun create cluster
kubectl apply -f manifests/crd-appconfig.yaml -f manifests/crd-newconfig.yaml -f manifests/namespaces.yaml
kubectl apply -f manifests/crs-appconfig.yaml
kubectl get appconfigs -A -L tier
```

```console
NAMESPACE     NAME         IMAGE             MINREPLICAS   LOGLEVEL   AGE   TIER
platform-01   api-server   myapp/api:2.2.0   6             warn       2s    web
platform-02   api-server   myapp/api:2.2.0   6             warn       2s    web
platform-03   api-server   myapp/api:2.2.0   6             warn       2s    web
platform-04   api-server   myapp/api:2.2.0   6             warn       2s    batch
...
platform-10   api-server   myapp/api:2.2.0   6             warn       1s
```

Ten CRs: `web`, `batch`, `internal` tiers, and one with no tier. `web` and
`internal` are `Ready`; the rest are `Pending`.

### 2. Validate and dry-run

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

The `scope:` line is the selection the run used. The dry-run applies each
destination with server-side dry-run, so `--verbose` shows the spec the API
server would store, defaults included:

```bash
inrun migrate --dry-run --verbose --limit 1
```

### 3. Narrow from the CLI — never widen

CLI flags combine with `migrate.selection`: namespaces intersect, selectors
AND, names must agree.

```bash
inrun migrate --dry-run -n platform-02,platform-07
```

```console
◆  AppConfig → NewConfig  1 CR
   scope: namespaces platform-02,platform-07 · selector tier=web
  ✓ platform-02/api-server  would migrate to NewConfig
```

`--selector tier=internal` gives `tier=internal,tier=web` and selects nothing.
A namespace outside a declared `selection.namespaces` is an error.

### 4. The `when` condition

```bash
kubectl patch appconfig api-server -n platform-03 --type merge -p '{"status":{"phase":"Pending"}}'
inrun migrate --dry-run
```

```console
  ✓ platform-01/api-server  would migrate to NewConfig
  ✓ platform-02/api-server  would migrate to NewConfig
  – platform-03/api-server  (not selected: migrate.selection when/or)

  ✓ 2/3 would be migrated · 1 not selected
```

### 5. Migrate

```bash
inrun migrate
kubectl get newconfigs -A -L migrated-from,tier
```

```console
NAMESPACE     NAME         IMAGE             MINREPLICAS   LOGLEVEL   AGE   MIGRATED-FROM   TIER
platform-01   api-server   myapp/api:2.2.0   6             warn       1s    appconfig       web
platform-02   api-server   myapp/api:2.2.0   6             warn       1s    appconfig       web
```

The sources are still there: prune is off. Each destination carries its
snapshot:

```bash
kubectl get newconfig api-server -n platform-01 \
  -o jsonpath='{.metadata.annotations.inrun\.dev/migrate-kind-snapshot}' | jq
```

```json
{
  "source": {
    "apiVersion": "demo.inrun.dev/v1",
    "kind": "AppConfig",
    "metadata": { "name": "api-server", "namespace": "platform-01", "labels": { "app": "demo", "tier": "web" } },
    "spec": { "image": "myapp/api:2.2.0", "observability": { "logging": { "level": "warn" } }, "scaling": { "min": 6 } }
  },
  "destination": { "apiVersion": "demo.inrun.dev/v1", "kind": "NewConfig", "metadata": { "name": "api-server", "namespace": "platform-01" } },
  "pruned": false
}
```

Run it again and nothing changes:

```console
  – platform-01/api-server  (already migrated)
  – platform-02/api-server  (already migrated)
  – platform-03/api-server  (not selected: migrate.selection when/or)

  ✓ 0/3 migrated · 2 already done · 1 not selected
```

### 6. Prune

Set platform-03 back to `Ready` and uncomment `prune: true`. A forward run
with prune first counts what it would delete and asks:

```bash
kubectl patch appconfig api-server -n platform-03 --type merge -p '{"status":{"phase":"Ready"}}'
inrun migrate
```

```console
⚠ migrate.prune is on. These source CRs are deleted after their destination is written:
    AppConfig → NewConfig  3 sources

Delete 3 source CRs after migrating? [y/N]:
```

Without a terminal it stops and asks for `--yes`; `--dry-run` never asks.

```bash
inrun migrate --yes
```

```console
  ✓ platform-01/api-server  already migrated; source pruned
  ✓ platform-02/api-server  already migrated; source pruned
  ✓ platform-03/api-server  migrated to NewConfig; source pruned

  ✓ 1/3 migrated · 3 sources pruned
```

A source is pruned only after its destination holds every source field. The
delete is conditional on the source's resourceVersion: if anything wrote to
the source after it was read, the prune is refused and the CR is reported, so
a concurrent write is never lost.

Pruning is a plain delete, so Kubernetes garbage-collects anything the source
owns. Have the destination's reconciler take over those children first.

### 7. Roll back

```bash
inrun migrate rollback --dry-run
inrun migrate rollback --name api-server -n platform-01
```

```console
◆  AppConfig → NewConfig  1 CR
   scope: namespaces platform-01 · name api-server · selector tier=web
  ✓ platform-01/api-server  rolled back to AppConfig; NewConfig deleted
```

Rollback reads the destinations whose snapshot names `AppConfig` as the
source, and matches the selection against the recorded source. It recreates
the source first and deletes the destination only after that succeeds. A
destination edited after migration is reported and left in place.

### 8. Clean

Once the move is final, drop the snapshots:

```bash
inrun migrate clean --dry-run
inrun migrate clean
```

After clean, rollback has nothing to act on.

### 9. Callers never noticed

Run the gateway; `--migrate` runs the forward migration at startup (prune
included when declared — there is no prompt in the gateway):

```bash
DEV_TOKEN=dev-token inrun gate run --migrate
```

```bash
curl -s -X POST localhost:8080/api/v1/apply \
  -H "Authorization: Bearer dev-token" -H "Content-Type: application/json" \
  -d '{"target":"my-new-api-server","image":"myapp/api:3.0.0","replicas":4,"logLevel":"info"}'
```

```console
{"accepted":true,"name":"api-server","namespace":"platform-09","kind":"NewConfig",...}
```

The same flat intent lands in `NewConfig` at `scaling.min` and
`observability.logging.level`.

## Clean up

```bash
./cleanup.sh
```
