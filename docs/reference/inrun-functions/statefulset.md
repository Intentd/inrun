# StatefulSet Notes

StatefulSet notes read rollout revision state and the PVC inventory embedded by `enrich: [pvcs]`.

---

## Reference

| Note | Description |
|------|-------------|
| `statefulSetCurrentRevision` | Returns `status. |
| `statefulSetUpdateRevision` | Returns `status. |
| `statefulSetPVCCount` | Returns the number of PVCs embedded in `_pvcs`. |

## Examples

```yaml
# statefulSetCurrentRevision
- path: currentRevision
  value: "{{ statefulSetCurrentRevision .children.statefulset }}"
# → "my-sts-6d8f4b9c5"

# statefulSetUpdateRevision
- path: updateRevision
  value: "{{ statefulSetUpdateRevision .children.statefulset }}"
# → "my-sts-7a2c1d4f8"

# statefulSetPVCCount
- path: pvcCount
  value: "{{ statefulSetPVCCount .children.statefulset }}"
# → 3
```
