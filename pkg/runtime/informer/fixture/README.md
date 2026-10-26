# informer fixture

Living fixture for `pkg/runtime/informer`. Verifies that the namespace filter
gates reconciliation at the queue layer — not the reconciler layer.

A CR in an allowed namespace is reconciled and reaches `Running`. A CR in a
restricted namespace is accepted by the API server but the informer silently
drops the event: the reconciler never runs, no resources are created, and the
status is never set.

No gateway required.

```bash
inrun simulate -f pkg/runtime/informer/fixture/simulate.yaml
inrun e2e     -f pkg/runtime/informer/fixture/e2e.yaml
```
