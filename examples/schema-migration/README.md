# schema-migration

The gateway decouples callers from the CRD schema. A caller submits flat
values — `replicas: 3`, `logLevel: info` — and the serve layer writes them to
wherever the CRD declares they live, whether that is `spec.replicas` today or
`spec.scaling.min` after a schema upgrade. The caller's interface never
changes.

`migrate.from` extends this to historical data. CRs that were created before a
schema change still hold values in the old paths. The migrator reads each old
path, runs the same transform the gateway uses on every apply, and writes the
result to the new path. One declaration drives both the live apply path and the
one-time backfill.

## Examples

### [fanout](fanout/README.md)

`spec.schedule` changes from a flat cron string to a five-field structured
object. The gateway always accepted a single cron string from callers; after
the upgrade it fans that string out to five spec fields. `migrate.from` backfills
the CRs that were written before the upgrade.

### [relocation](relocation/README.md)

`spec.replicas` and `spec.logLevel` move to new nested paths:
`spec.scaling.min` and `spec.observability.logging.level`. The `path:` declaration
redirects where the gateway writes; `migrate.from` names where old values live.
Shows the **transitional CRD** pattern (required to prevent Kubernetes from
pruning old fields before migration reads them) and the contrast between a CR
created via the gateway — which lands in the correct shape from the start — and
one created directly with kubectl against the old schema.
