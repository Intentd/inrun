# relocation

An `AppConfig` operator where two fields move to new nested paths:

- `spec.replicas` (flat int) → `spec.scaling.min`
- `spec.logLevel` (flat string) → `spec.observability.logging.level`

Callers never know this happened. Before and after the migration, they submit
the same flat values through the gateway:

```console
curl -X POST http://localhost:8080/api/v1/apply \
  -H "Authorization: Bearer $DEV_TOKEN" \
  -d '{"target":"platform-09/api-server","image":"myapp/api:2.2.0","replicas":3,"logLevel":"info"}'
```

The gateway reads `path: scaling.min` and `path: observability.logging.level`
from the serve declarations and writes to those paths. The CR lands in the
correct shape from the start — no migration needed for it. `migrate.from`
handles only the CRs that were written before the schema changed.

## The schema change

**Old** — flat fields at the top of spec:

```yaml
spec:
  image: "myapp/api:2.1.0"
  replicas: 3
  logLevel: "info"
```

**New** — fields moved to nested objects:

```yaml
spec:
  image: "myapp/api:2.1.0"
  scaling:
    min: 3
  observability:
    logging:
      level: "info"
```

The serve declarations that drive both the live apply path and the migration:

```yaml
replicas:
  path: scaling.min        # write target — used by gateway on every apply
  migrate:
    from: replicas         # read source — used by migrator for old CRs

logLevel:
  path: observability.logging.level
  migrate:
    from: logLevel
```

## The transitional CRD

Kubernetes structural schema pruning removes any field not declared in the CRD the moment a new CRD version is applied. If you applied a CRD that only declares `scaling.min` and not `replicas`, every stored CR would lose its `replicas` value before migration could read it.

The solution is a **transitional CRD** (`manifests/crd-transitional.yaml`) that declares both the old and new fields simultaneously. Migration runs against this CRD, reads the old fields, writes the new ones, then the **final CRD** (`manifests/crd-final.yaml`) removes the old declarations safely.

Three-step pattern for any field relocation: `old CRD → transitional CRD → migrate → final CRD`

## Edge cases covered

- **8 old / 1 gateway / 1 kubectl** — platforms 09 and 10 are created after the
  upgrade. Platform-09 is created via the gateway (flat values in, correct
  nested spec out). Platform-10 is created directly with kubectl. The migrator
  treats both as no-ops.
- **Partial old schema** — platform-05's CR has `replicas` but no `logLevel`.
  The migrator moves `replicas` and silently skips the absent `logLevel`. The
  output shows `migrated: replicas` only.
- **Idempotency** — re-running after migration shows all 10 CRs as
  `already migrated`.
- **Drift** — platform-06's `scaling.min` is edited after the migration. Its
  rollback restores `logLevel` and skips `replicas`, keeping the edit and the
  snapshot entry; migrating it again keeps that entry too.

---

## Walkthrough

### 1. Apply the old CRD and namespaces

```bash
kubectl apply -f manifests/old-crd.yaml
kubectl apply -f manifests/namespaces.yaml
```

### 2. Create CRs with the old flat schema

```bash
kubectl apply -f manifests/old-crs.yaml
```

### 3. Apply the transitional CRD

Adds the new nested fields while keeping the old flat ones. Both old and new
CRs can coexist against this schema.

```bash
kubectl apply -f manifests/crd-transitional.yaml
```

### 4. Create two new-form CRs — one via the gateway, one via kubectl

Validate the Blueprint

```bash
inrun validate
```

Start the gateway:

```bash
DEV_TOKEN=dev-token inrun gate run
```

In a second terminal, submit platform-09 through the gateway using the flat caller interface. The gateway writes directly to `spec.scaling.min` and `spec.observability.logging.level`:

```console
curl -s -X POST http://localhost:8080/api/v1/apply \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "target": "my-api-server",
    "image": "myapp/api:2.2.0",
    "replicas": 3,
    "logLevel": "info"
  }'
```

The gateway uses the target `my-api-server` as configured in **[blueprint.yaml](blueprint.yaml)** and lands on the configured namespace

Platform-10 is created directly with kubectl using the new schema YAML:

```bash
kubectl apply -f manifests/new-crs.yaml
```

Stop the gateway (`ctrl-c`) and continue.

### 5. Observe the mixed state

```bash
kubectl get appconfigs.demo.inrun.dev -A
```

The transitional CRD shows both old and new printer columns. Platforms 01–08
have data under `REPLICAS`/`LOGLEVEL`. Platforms 09–10 — created via two
different paths — both land correctly under `MINREPLICAS`/`NEWLOGLEVEL`:

```console
NAMESPACE     NAME       IMAGE             REPLICAS  MINREPLICAS  LOGLEVEL  NEWLOGLEVEL
platform-01   api-server myapp/api:2.1.0   3                      info
platform-02   api-server myapp/api:2.1.0   5                      warn
platform-03   api-server myapp/api:1.9.0   2                      debug
platform-04   api-server myapp/api:2.1.0   10                     error
platform-05   api-server myapp/api:2.0.0   1
platform-06   api-server myapp/api:2.1.0   4                      info
platform-07   api-server myapp/api:2.1.0   7                      warn
platform-08   api-server myapp/api:2.1.0   3                      info
platform-09   api-server myapp/api:2.2.0              3                     info
platform-10   api-server myapp/api:2.2.0              6                     warn
```

### 6. Dry-run the migration

```bash
inrun migrate --dry-run
```

8 would be migrated. Platforms 09 and 10 are already done regardless of how they were created. Platform-05 shows only `replicas`:

```console
⟳  inrun migrate  [dry-run]
  blueprint: blueprint.yaml

◆  AppConfig  10 CRs
  ✓ platform-01/api-server  would migrate: replicas, logLevel
  ✓ platform-02/api-server  would migrate: replicas, logLevel
  ✓ platform-03/api-server  would migrate: replicas, logLevel
  ✓ platform-04/api-server  would migrate: replicas, logLevel
  ✓ platform-05/api-server  would migrate: replicas
  ✓ platform-06/api-server  would migrate: replicas, logLevel
  ✓ platform-07/api-server  would migrate: replicas, logLevel
  ✓ platform-08/api-server  would migrate: replicas, logLevel
  –  platform-09/api-server  (already migrated)
  –  platform-10/api-server  (already migrated)

  ✓ 8/10 would be migrated · 2 already done
```

### 7. Run the migration

```bash
inrun migrate
```

### 8. Verify

```bash
kubectl get appconfigs.demo.inrun.dev -A
```

```console
NAMESPACE     NAME       IMAGE             MINREPLICAS  LOGLEVEL  AGE
platform-01   api-server myapp/api:2.1.0   3            info      ...
platform-02   api-server myapp/api:2.1.0   5            warn      ...
platform-03   api-server myapp/api:1.9.0   2            debug     ...
platform-04   api-server myapp/api:2.1.0   10           error     ...
platform-05   api-server myapp/api:2.0.0   1                      ...
platform-06   api-server myapp/api:2.1.0   4            info      ...
platform-07   api-server myapp/api:2.1.0   7            warn      ...
platform-08   api-server myapp/api:2.1.0   3            info      ...
platform-09   api-server myapp/api:2.2.0   3            info      ...
platform-10   api-server myapp/api:2.2.0   6            warn      ...
```

Inspect platform-09 — created via the gateway, spec is identical to any manually written new-form CR:

```bash
kubectl get appconfig api-server -n platform-09 -o jsonpath='{.spec}' | jq
```

```json
{
  "image": "myapp/api:2.2.0",
  "scaling": { "min": 3 },
  "observability": { "logging": { "level": "info" } }
}
```

### 9. Idempotency — run again

```bash
inrun migrate
```

```
◆  AppConfig  10 CRs
  –  platform-01/api-server  (already migrated)
  –  platform-02/api-server  (already migrated)
  –  platform-03/api-server  (already migrated)
  –  platform-04/api-server  (already migrated)
  –  platform-05/api-server  (already migrated)
  –  platform-06/api-server  (already migrated)
  –  platform-07/api-server  (already migrated)
  –  platform-08/api-server  (already migrated)
  –  platform-09/api-server  (already migrated)
  –  platform-10/api-server  (already migrated)

  ✓ 0/10 migrated · 10 already done
```

### 10. Rollback
Assuming an error occured during the migration and you want to rollback, you can rollback a particular CR by name/namespace/selector

#### Dry run the rollback

```bash
inrun migrate rollback --dry-run
```

```
⟳  inrun migrate rollback  [dry-run]
  blueprint: blueprint.yaml

◆  AppConfig  10 CRs
  ✓ platform-01/api-server  would roll back: replicas, logLevel
  ✓ platform-02/api-server  would roll back: replicas, logLevel
  ✓ platform-03/api-server  would roll back: replicas, logLevel
  ✓ platform-04/api-server  would roll back: replicas, logLevel
  ✓ platform-05/api-server  would roll back: replicas
  ✓ platform-06/api-server  would roll back: replicas, logLevel
  ✓ platform-07/api-server  would roll back: replicas, logLevel
  ✓ platform-08/api-server  would roll back: replicas, logLevel
  – platform-09/api-server  (no snapshot)
  – platform-10/api-server  (no snapshot)

  ✓ 8/10 would be rolled back · 2 no snapshot

```
platform-09 and platform-10 cannot be rolled back because they were never migrated in the first place.
Snapshots are injected by the migrator as `before` and `after` annotations during migration. The record travels with the resource.

#### View a snapshot

```bash
kubectl get appconfig api-server -n platform-07 -o jsonpath='{.metadata.annotations.inrun\.dev/migrate-field-snapshot}' | jq
```

```json
{
  "logLevel": {
    "after": {
      "logLevel": null,
      "observability.logging.level": "warn"
    },
    "before": {
      "logLevel": "warn",
      "observability.logging.level": null
    }
  },
  "replicas": {
    "after": {
      "replicas": null,
      "scaling.min": 7
    },
    "before": {
      "replicas": 7,
      "scaling.min": null
    }
  }
}
```

#### Drift: a later edit is never overwritten

Rollback restores a field only while it still holds exactly what the migration wrote. Someone scales platform-06
after the migration, `scaling.min` from 4 to 6, and leaves its log level alone:

```bash
kubectl patch appconfig api-server -n platform-06 --type merge -p '{"spec":{"scaling":{"min":6}}}'
```

Dry run its rollback:

```bash
inrun migrate rollback --name api-server --namespace platform-06 --dry-run
```

```
⟳  inrun migrate rollback  [dry-run]
  blueprint: blueprint.yaml
  namespace: platform-06
  name: api-server

◆  AppConfig  1 CR
  ✓ platform-06/api-server  would roll back: logLevel
  ~ platform-06/api-server  replicas: drifted — modified after migration, skipped

  ✓ 1/1 would be rolled back · 1 drifted
```

One CR, two outcomes: `logLevel` still holds what the migration wrote, so it rolls back; `replicas` was changed
since, so it is skipped and the edit survives. Roll back for real:

```bash
inrun migrate rollback --name api-server --namespace platform-06
```

```bash
kubectl get appconfig api-server -n platform-06 -o jsonpath='{.spec}' | jq
```

```json
{
  "image": "myapp/api:2.1.0",
  "logLevel": "info",
  "observability": {
    "logging": {}
  },
  "scaling": {
    "min": 6
  }
}
```

`logLevel` is back at its old path; `scaling.min` keeps the edit. The snapshot keeps the entry for the skipped field,
so nothing about the original value is lost:

```bash
kubectl get appconfig api-server -n platform-06 -o jsonpath='{.metadata.annotations.inrun\.dev/migrate-field-snapshot}' | jq
```

```json
{
  "replicas": {
    "after": {
      "replicas": null,
      "scaling.min": 4
    },
    "before": {
      "replicas": 4,
      "scaling.min": null
    }
  }
}
```

Migrate platform-06 again so every CR is in the new form before the final CRD:

```bash
inrun migrate --name api-server --namespace platform-06
```

`logLevel` moves again; `scaling.min` stays at 6. The snapshot now holds both entries: the new one for `logLevel`
and the one rollback kept for `replicas`, so the original value is still recoverable:

```json
{
  "logLevel": {
    "after": {
      "logLevel": null,
      "observability.logging.level": "info"
    },
    "before": {
      "logLevel": "info",
      "observability.logging.level": null
    }
  },
  "replicas": {
    "after": {
      "replicas": null,
      "scaling.min": 4
    },
    "before": {
      "replicas": 4,
      "scaling.min": null
    }
  }
}
```

#### Rollback platform-07

Observe current state:

```bash
kubectl get appconfig api-server -n platform-07 -o jsonpath='{.spec}' | jq
```

```json
{
  "image": "myapp/api:2.1.0",
  "observability": {
    "logging": {
      "level": "warn"
    }
  },
  "scaling": {
    "min": 7
  }
}
```

Dry run the rollback for this CR:

```bash
inrun migrate rollback --name api-server --namespace platform-07 --dry-run --verbose
# replace verbose with -ojson for machine-readable output
```

```
◆  AppConfig  1 CR
  ✓ platform-07/api-server  would roll back: replicas, logLevel
    spec:
      image: myapp/api:2.1.0
      logLevel: warn
      observability:
        logging:
          level: null
      replicas: 7
      scaling:
        min: null

  ✓ 1/1 would be rolled back
```

Apply the rollback:

```bash
inrun migrate rollback --name api-server --namespace platform-07
```

Observe rolled-back state:

```bash
kubectl get appconfig api-server -n platform-07 -o jsonpath='{.spec}' | jq
```

```json
{
  "image": "myapp/api:2.1.0",
  "logLevel": "warn",
  "observability": {
    "logging": {}
  },
  "replicas": 7,
  "scaling": {}
}
```

Re-apply the migration
```bash
inrun migrate --name api-server --namespace platform-07
```

### 11. Apply the final CRD

All CRs are in the new form. Pruning is safe.

```bash
kubectl apply -f manifests/crd-final.yaml
```

### 12. Clean up

Optional: Clean up migration metadata after the final CRD has been applied

Dry run:
```bash
inrun migrate clean --dry-run
```

Clean:
```bash
inrun migrate clean
```

Cleanup other resources
```bash
chmod +x cleanup.sh && ./cleanup.sh
```

---

## Running inrun gateway with migration on startup

After cleanup, fold migration into startup. The gateway migrates before serving.

```bash
kubectl apply -f manifests/crd-transitional.yaml
kubectl apply -f manifests/namespaces.yaml
kubectl apply -f manifests/old-crs.yaml
kubectl apply -f manifests/new-crs.yaml
```

### Options
- Use the full stack (local development): starts runtime, gateway, console

```bash
DEV_TOKEN=dev-token inrun --migrate
# gateway log shows per-CR migration before the first request is served
```

- Start the gateway only: This is how it'd run in a helm deployed one

```bash
DEV_TOKEN=dev-token inrun gate run --migrate
```

### Apply the final CRD
```bash
kubectl apply -f manifests/crd-final.yaml
```
