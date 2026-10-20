# Replica

Read the replica counts of a workload (desired, ready, available and updated) and whether all replicas are ready or a rollout has completed.

## Reference

| Function | Description |
|----------|-------------|
| [`allReplicasReady`](#allreplicasready) | Returns `true` when `status.readyReplicas == spec.replicas`. |
| [`availableReplicas`](#availablereplicas) | Returns `status.availableReplicas` as `int`. |
| [`desiredReplicas`](#desiredreplicas) | Returns `spec.replicas` as `int`. |
| [`readyReplicas`](#readyreplicas) | Returns `status.readyReplicas` as `int`. |
| [`rolloutComplete`](#rolloutcomplete) | Reports whether a workload's rollout has finished, by the rules kubectl rollout status uses for its kind. |
| [`updatedReplicas`](#updatedreplicas) | Returns `status.updatedReplicas` as `int` — pods running the current pod template. |

## Functions

### allReplicasReady

`allReplicasReady` returns `true` when `status.readyReplicas == spec.replicas`. The canonical rollout-complete gate. Returns `true` when scaled to zero (desired=0 and ready=0).

```text
{{ allReplicasReady .deployment }}  → true
```

### availableReplicas

`availableReplicas` returns `status.availableReplicas` as `int`. Returns `0` when absent.

```text
{{ availableReplicas .deployment }}  → 3
```

### desiredReplicas

`desiredReplicas` returns `spec.replicas` as `int`. Returns `1` when not set (Kubernetes default).

```text
{{ desiredReplicas .deployment }}  → 3
```

### readyReplicas

`readyReplicas` returns `status.readyReplicas` as `int`. Returns `0` when absent.

```text
{{ readyReplicas .deployment }}  → 3
```

### rolloutComplete

`rolloutComplete` reports whether a workload's rollout has finished, by the rules kubectl rollout status uses for its kind. In every case the status must describe the current spec (status.observedGeneration at least metadata.generation), so a change the controller has not seen yet is not complete.

- Deployment: every replica runs the new template, no old replica is left, and the new replicas are available.
- StatefulSet: every replica is ready and, with a partition, the replicas from the partition up are updated; without one, the current revision is the update revision.
- DaemonSet: every scheduled node runs the new template and is available.
- Any other kind: updated replicas equal desired replicas.

An object without kind, such as a typed object from a list call converted to a map, is read by its shape: a DaemonSet's status, a StatefulSet's revisions or a Deployment's strategy.

```text
{{ rolloutComplete .deployment }}  → true
```

### updatedReplicas

`updatedReplicas` returns `status.updatedReplicas` as `int` — pods running the current pod template. When `updatedReplicas == desiredReplicas`, the rollout is complete.

```text
{{ updatedReplicas .deployment }}  → 3
```
