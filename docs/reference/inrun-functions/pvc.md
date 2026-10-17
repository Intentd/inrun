# PVC Notes

PVC notes read PersistentVolumeClaim phase, capacity, storage class, and access modes. Enriched notes navigate the bound PV (via `enrich: [pvc]`) or the StorageClass object (via `enrich: [storageclass]`).

---

## Reference

| Note | Description |
|------|-------------|
| `pvcBound` | Returns `true` when `status. |
| `pvcPhase` | Returns `status. |
| `pvcCapacity` | Returns `spec. |
| `pvcStorageClass` | Returns `spec. |
| `pvcAccessModes` | Returns a comma-separated list of `spec. |
| `pvcProvisioner` | Returns the provisioner annotation from the bound PV (`pv. |
| `pvcVolumeMode` | Returns the `volumeMode` of the bound PV — `"Filesystem"` or `"Block"`. |
| `pvcStorageClassProvisioner` | Returns `_storageClass. |
| `pvcStorageClassReclaimPolicy` | Returns `_storageClass. |

## Examples

```yaml
# pvcBound
when:
  - field: "{{ pvcBound .children.pvc }}"
    equals: "true"

# pvcPhase
- path: pvcPhase
  value: "{{ pvcPhase .children.pvc }}"
# → "Bound"

# pvcCapacity
- path: capacity
  value: "{{ pvcCapacity .children.pvc }}"
# → "10Gi"

# pvcStorageClass
- path: storageClass
  value: "{{ pvcStorageClass .children.pvc }}"
# → "standard"

# pvcAccessModes
- path: accessModes
  value: "{{ pvcAccessModes .children.pvc }}"
# → "ReadWriteOnce"

# pvcProvisioner
- path: provisioner
  value: "{{ pvcProvisioner .children.pvc }}"
# → "ebs.csi.aws.com"

# pvcVolumeMode
- path: volumeMode
  value: "{{ pvcVolumeMode .children.pvc }}"
# → "Filesystem"

# pvcStorageClassProvisioner
- path: scProvisioner
  value: "{{ pvcStorageClassProvisioner .children.pvc }}"
# → "ebs.csi.aws.com"

# pvcStorageClassReclaimPolicy
- path: reclaimPolicy
  value: "{{ pvcStorageClassReclaimPolicy .children.pvc }}"
# → "Delete"
```
