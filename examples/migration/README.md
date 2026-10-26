# migration

The gateway decouples callers from the CRD schema. A caller submits flat values — `replicas: 3`, `logLevel: info` — and the serve layer writes them to wherever the CRD declares they live, whether that is `spec.replicas` today or `spec.scaling.min` after a schema upgrade. The caller's interface never changes.

`migrate.from` extends this to historical data. CRs that were created before a schema change still hold values in the old paths. The migrator reads each old path, runs the same transform the gateway uses on every apply, and writes the result to the new path. One declaration drives both the live apply path and the one-time backfill.

`migrate.to` goes one step further: it moves CRs to a different CRD altogether. Because callers only ever see intents, the CRD is storage the platform can replace.

## Examples

Two levels, one folder each:

- **field/** — `serve.fields[*].migrate.from`: values move to new paths inside the same CRD. The gateway already writes the new shape; the migration backfills CRs written before the change.
- **kind/** — `migrate.to`: CRs move to a different CRD (new kind, group or scope). Each destination keeps a snapshot, so the move rolls back.

`migrate.selection` scopes either level: label selector, namespaces, name, and `when`/`or` conditions on the whole object. CLI flags narrow it, never widen it.

### field/[fanout](field/fanout/README.md)

`spec.schedule` changes from a flat cron string to a five-field structured object. The gateway always accepted a single cron string from callers; after the upgrade it fans that string out to five spec fields. `migrate.from` backfills the CRs that were written before the upgrade.

### field/[relocation](field/relocation/README.md)

`spec.replicas` and `spec.logLevel` move to new nested paths: `spec.scaling.min` and `spec.observability.logging.level`. The `path:` declaration redirects where the gateway writes; `migrate.from` names where old values live. Shows the **transitional CRD** pattern (required to prevent Kubernetes from pruning old fields before migration reads them) and the contrast between a CR created via the gateway — which lands in the correct shape from the start — and one created directly with kubectl against the old schema.

### kind/[rename](kind/rename/README.md)

`AppConfig` moves to a new kind, `NewConfig`. Shows selection narrowed from the CLI, a `when` on status, validate-time schema checks from both crdFiles, prune with confirmation, rollback that recreates pruned sources, and clean. Callers keep sending the same intents through the gateway.

### kind/[rescope](kind/rescope/README.md)

`AppConfig` moves from a namespaced CRD to a cluster-scoped one. Names that were unique per namespace must be unique cluster-wide: collisions are caught even in a dry-run and never overwrite. Also shows `carryStatus` and the reverse direction's `migrate.to.namespace`.

## Run the complete migration suite

From the repository root:

```bash
inrun e2e --report
```