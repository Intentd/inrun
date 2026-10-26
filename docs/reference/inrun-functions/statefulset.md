# StatefulSet

Read what is particular to a StatefulSet: its current and update revisions, and the partition of a staged rolling update. Replica counts, rollout completion and pods have their own functions.

## Reference

| Function | Description |
|----------|-------------|
| [`statefulSetCurrentRevision`](#statefulsetcurrentrevision) | Returns `status.currentRevision` — the pod template hash of currently running pods, or `""` before the first rollout. |
| [`statefulSetPartition`](#statefulsetpartition) | Returns the partition of a StatefulSet's rolling update (spec.updateStrategy.rollingUpdate.partition): pods with an ordinal at or above it are updated, the rest keep the old revision. |
| [`statefulSetUpdateRevision`](#statefulsetupdaterevision) | Returns `status.updateRevision` — the hash of the pending update; equal to `currentRevision` when the rollout is complete. |

## Functions

### statefulSetCurrentRevision

`statefulSetCurrentRevision` returns `status.currentRevision` — the pod template hash of currently running pods, or `""` before the first rollout.

```text
{{ statefulSetCurrentRevision .statefulset }}  → "db-6d8f4b9c5"
```

### statefulSetPartition

`statefulSetPartition` returns the partition of a StatefulSet's rolling update (spec.updateStrategy.rollingUpdate.partition): pods with an ordinal at or above it are updated, the rest keep the old revision. 0 when unset.

```text
{{ statefulSetPartition .statefulset }}  → 2
```

### statefulSetUpdateRevision

`statefulSetUpdateRevision` returns `status.updateRevision` — the hash of the pending update; equal to `currentRevision` when the rollout is complete.

```text
{{ statefulSetUpdateRevision .statefulset }}  → "db-7a2c1d4f8"
```
