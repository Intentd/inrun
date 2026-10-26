# filter fixture

Living fixture for `preReconcile.enqueueGate`. Verifies that the enqueue gate
drops objects at the informer layer — before they enter the work queue —
when `spec.active` is false.

The key distinction from `preReconcile.reconcileGate`: the coordinator never sees
the object, so CRD health stays **healthy** (not `gated`). Queue depth stays 0.
The reconciler is never called.

```bash
inrun validate -f pkg/runtime/informer/fixture/filter/blueprint.yaml
inrun simulate -f pkg/runtime/informer/fixture/filter/simulate.yaml
inrun e2e      -f pkg/runtime/informer/fixture/filter/e2e.yaml
```
