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

`migrate.from` is a spec path: `replicas`, `spec.replicas` and `.spec.replicas` name the same field, and a path under `status` or `metadata` is refused.

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

Each selected source is rebuilt as a destination CR and applied. Before writing, a dry-run apply refuses a destination that would drop or change a source field, and name collisions fail rather than overwrite (a namespaced to cluster-scoped move makes names unique cluster-wide). `migrate.to.copyStatus` copies the status across.

## Lifecycle: what happens to each source

By default a source stays where it is once its destination is written. `migrate.lifecycle` deletes it:

```yaml
migrate:
  to:
    crdFile: ./manifests/crd-newconfig.yaml
  lifecycle:
    prune: true
    children: reparent        # or orphan, or delete
    finalizers:
      - example.io/cleanup
```

- `prune` deletes a source only once the destination holds every source field, and only if the source hasn't changed since it was read.
- `children` is what happens to the objects the source owns (by owner reference): `reparent` moves them to the destination, `orphan` keeps them without an owner, `delete` deletes them with the source. A source with children and no policy is not pruned.
- `finalizers` are the finalizers the migration may remove before deleting a source. A source with any other finalizer is not pruned.

`children` and `finalizers` need `prune`, and validation refuses Kubernetes' own finalizers. The CLI asks before pruning unless `--yes`; `--dry-run` lists what would be deleted.

The gateway never prunes. When it migrates at startup (`--migrate`), it moves the sources, skips the lifecycle and logs a warning for each CRD that declares `prune`, so its role needs no `delete` on sources. Pruning is a step a person runs with `inrun migrate`.

## Needs: migrations in order

`migrate.needs` names the CRDs (their `spec.crds` keys) whose migrations run first: the same CRs moved twice, or a destination another migration creates.

```yaml
spec:
  crds:
    appconfig:
      migrate:
        to:
          crdFile: ./manifests/crd-newconfig.yaml
    newconfig:
      crdFile: ./manifests/crd-newconfig.yaml
      serve:
        fields:
          replicas:
            path: scaling.min
            migrate:
              from: replicas
      migrate:
        needs: [appconfig]
```

Every run goes in that order, and rollback in reverse. A run limited with `--kind` still checks the others: it refuses to migrate while a migration its kind needs is pending, and to roll back while a migration that needs it is applied. Each name must be a CRD that declares a migration, and the needs can't form a cycle. When field migrations of one kind need each other, the generated transitional CRD covers every path along the chain.

## Selection

`migrate.selection` scopes either level: `namespaces`, `name`, a label `selector`, and `when` / `or` conditions on the object. The CLI's `--namespace`, `--name` and `--selector` can narrow it, never widen it.

## Running it

```bash
inrun migrate --dry-run
inrun migrate
```

`--kind` limits a run to one CRD and `-o json` gives machine-readable output. Migration can also run before anything is served: `inrun --migrate` (or `inrun gate run --migrate`, as the gateway runs in a cluster) migrates first, without the lifecycle.

`--set` changes a kind's `migrate` block for one run without editing the Blueprint, keys relative to it. It needs `--kind` for one kind or `--all` (`-A`) for every kind that declares a migration, and the result is validated like the Blueprint:

```bash
inrun migrate --kind AppConfig --set lifecycle.prune=true --yes
```

## Rollback

Every migrated CR carries a snapshot of what it held before: `inrun.dev/migrate-field-snapshot` for fields, a snapshot on the destination for kinds. The record travels with the resource.

```bash
inrun migrate rollback --dry-run
inrun migrate rollback --name api-server --namespace platform-07
```

Rollback restores a field only while it still holds exactly what the migration wrote. A value someone changed since is reported as drifted and left alone, and its snapshot entry is kept, so nothing is lost either way. A kind rollback recreates the source before deleting the destination.

A destination the migration did not create is different: the gateway or direct kubectl wrote it first, and the migration finds it already there. If it holds different values from the source, such as a newer intent, it is drift: the migration leaves both alone. If it holds exactly what the migration would write, it counts as migrated, and before pruning that source the migration records the source on the destination, so it can still be rolled back. Rollback then restores the source and **keeps** the destination, since the migration didn't write it, removes the migration's record from it, and reports it as kept: not created by the migration.

`inrun migrate clean` removes the snapshots once you no longer need them.

## Try it

```bash
inrun init --pack migration
```
