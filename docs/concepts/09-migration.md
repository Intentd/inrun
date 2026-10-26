# Migration

Callers send intents, not CRs, so the CRD behind them is storage the platform can change. When it changes, the CRs already stored still hold the old shape. Migration brings them along, at two levels:

| Level | Declared as | Moves |
| --- | --- | --- |
| **Field** | `serve.fields[*].migrate.from` | values to a new path inside the same CRD (`spec.replicas` → `spec.scaling.min`) |
| **Kind** | `migrate.to` on a CRD entry | CRs to another CRD: a new kind, group or scope |

## Field: one declaration for writes and backfill

A served field's `path` says where the gateway writes it today; `migrate.from` says where older CRs still keep it:

```yaml
serve:
  fields:
    replicas:
      path: scaling.min       # under spec
      migrate:
        from: replicas
```

New intents land in the new shape from the start. `inrun migrate` reads each stored CR's old path, runs the same transform the gateway applies on every write, and writes the result to the new path. One declaration drives both the live writes and the one-time backfill, so they cannot disagree.

!!! note "Moving a field needs a transitional CRD"
    Kubernetes prunes fields a CRD schema doesn't declare, so a field move goes through a **transitional CRD** that has both paths. Migrate against it, then apply the final CRD.

## Kind: CRs to another CRD

```yaml
spec:
  crds:
    appconfig:
      crdFile: ./manifests/crd-appconfig.yaml
      migrate:
        to:
          crdFile: ./manifests/crd-newconfig.yaml
        selection:
          selector: tier=web
          when:
            - field: status.phase
              equals: Ready
```

Each selected source is rebuilt as a destination CR and applied. Before writing, a dry-run apply refuses a destination that would drop or change a source field, and name collisions fail rather than overwrite (a namespaced to cluster-scoped move makes names unique cluster-wide). `migrate.to.carryStatus` copies the status across. `prune` deletes a source only once the destination holds every source field, and only if the source hasn't changed since it was read.

## Selection

`migrate.selection` scopes either level: `namespaces`, `name`, a label `selector`, and `when` / `or` conditions on the object. The CLI's `--namespace`, `--name` and `--selector` can narrow it, never widen it.

## Running it

```bash
inrun migrate --dry-run
inrun migrate
```

`--kind` limits a run to one CRD and `-o json` gives machine-readable output. Migration can also run before anything is served: `inrun --migrate` (or `inrun gate run --migrate`, as the gateway runs in a cluster) migrates first.

## Rollback

Every migrated CR carries a snapshot of what it held before: `inrun.dev/migrate-field-snapshot` for fields, a snapshot on the destination for kinds. The record travels with the resource.

```bash
inrun migrate rollback --dry-run
inrun migrate rollback --name api-server --namespace platform-07
```

Rollback restores a field only while it still holds exactly what the migration wrote. A value someone changed since is reported as drifted and left alone, and its snapshot entry is kept, so nothing is lost either way. A kind rollback recreates the source before deleting the destination.

`inrun migrate clean` removes the snapshots once you no longer need them.

## Try it

```bash
inrun init --pack migration
```
