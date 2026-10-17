# 23 — StatefulSet Notes

StatefulSet notes read rollout revision state and the PVC inventory embedded by `enrich: [pvcs]`.

---

## Reference

### `statefulSetCurrentRevision`

Returns `status.currentRevision` — the pod template hash of currently running pods, or `""` before the first rollout.

Keywords: statefulset, revision, rollout, hash, string, current, template

```yaml
- path: currentRevision
  value: "{{ statefulSetCurrentRevision .children.statefulset }}"
# → "my-sts-6d8f4b9c5"
```

---

### `statefulSetUpdateRevision`

Returns `status.updateRevision` — the hash of the pending update; equal to `currentRevision` when the rollout is complete.

Keywords: statefulset, revision, rollout, update, string, hash, pending

```yaml
- path: updateRevision
  value: "{{ statefulSetUpdateRevision .children.statefulset }}"
# → "my-sts-7a2c1d4f8"
```

---

### `statefulSetPVCCount`

Returns the number of PVCs embedded in `_pvcs`. Requires `enrich: [pvcs]`.

Keywords: statefulset, pvc, storage, count, enriched, int, volumes

```yaml
- path: pvcCount
  value: "{{ statefulSetPVCCount .children.statefulset }}"
# → 3
```

---

## Quick reference

| Note | Signature | Returns | Enrichment |
|------|-----------|---------|------------|
| `statefulSetCurrentRevision` | `(obj any)` | `string` | none |
| `statefulSetUpdateRevision` | `(obj any)` | `string` | none |
| `statefulSetPVCCount` | `(obj any)` | `int` | `enrich: [pvcs]` |

---

**Next →** [24 — ReplicaSet Notes](24-replicaset.md)
