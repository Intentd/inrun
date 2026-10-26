# Runtime

The runtime is the process that reconciles. It watches the CRDs a Blueprint
declares and the children they write, queues each change for its CRD's
worker pool, runs the reconcile pass below, and applies the Result. One
replica leads (leader election); the rest wait to take over. It serves
`/blueprint` on its health port, which the [console](05-console.md) reads.

What follows happens on every reconcile, whatever the reconciler.

## One pass

1. **Gate.** `preReconcile` conditions decide whether the CR is reconciled now; a gated CR waits, which is not a failure. A CR being deleted is never gated.
2. **Prepare.** The CR is normalized and checked (namespace rules, validation), and `reconcile.external` calls run.
3. **Maintain.** Inrun's finalizer and managed labels are put on the CR; a `cleanup` that is due deletes the CR here.
4. **Reconcile.** `reconcile.args` are rendered and the reconciler returns a Result.
5. **Apply.** The Result's resources, labels and annotations are applied; with `reconcile.prune`, children it no longer returns are deleted.
6. **Status and events.** `emit.status` and the Result's status are merged onto the CR (even when the reconcile failed); a `Reconciled` or `ReconcileError` event is recorded.
7. **Next.** The Result's requeue, a conditional `reconcile.requeue`, a pending `cleanup`, or the next change decides when it runs again.

## Finalizers

Every CR Inrun manages carries `inrun.dev/cleanup`, plus any names in`runtime.finalizers.names`. `runtime.finalizers.remove: true` strips them instead (for CRs whose deletion should never wait. Useful during testing).

## Deletion

When a CR is deleted, its finalizer holds it and the reconciler gets a **finalizing** request (`req.Finalizing()`, or `"finalizing": true` for a remote reconciler):

- It holds deletion by asking to requeue: a Result's requeue, or a conditional `reconcile.requeue` (one with `when` / `or`). A requeue with only `after` is a refresh and never holds a deleting CR.
- Declarative CRDs run their `onDelete` templates; a failure keeps the finalizer and retries.
- Writes in a finalizing Result are ignored, so nothing recreates children of a CR on its way out.
- `runtime.finalizers.timeout` bounds the wait: after it, the CR is released anyway with a `FinalizeTimeout` event.

Once released, Inrun deletes the CR's children in every namespace (clusterscoped ones included, which owner references cannot reach), except kinds in `prune.keep`, then removes its finalizer. A `Finalized` event records it.

## Prune

`reconcile.prune.enabled` deletes children the Result applied before and no longer returns. Only objects carrying the CR's `inrun.dev/owner-uid` label and applied by the `inrun-reconciler` field manager are candidates; kinds in `prune.keep` never are. A failed reconcile prunes nothing, and an empty Result
prunes nothing unless `prune.allowEmpty` is set. Each deletion is a `Pruned` event. Prune can be set per CRD or per serve target.

## Events

| Event | When |
| --- | --- |
| `Reconciled` | A pass succeeded |
| `ReconcileError` | A pass failed (the error is in the message) |
| `ApplyFailed` | Applying the Result failed |
| `Pruned` | A child was deleted because the Result stopped returning it |
| `Finalized` | The finalizing reconcile finished and the CR was released |
| `FinalizeTimeout` | The CR was released after `runtime.finalizers.timeout` |

`kubectl describe` shows them on the CR.
