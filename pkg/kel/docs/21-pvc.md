# 21 — PVC Notes

PVC notes read PersistentVolumeClaim phase, capacity, storage class, and access modes. Enriched notes navigate the bound PV (via `enrich: [pvc]`) or the StorageClass object (via `enrich: [storageclass]`).

---

## Reference

### `pvcBound`

Returns `true` when `status.phase == "Bound"`.

Keywords: pvc, storage, bound, phase, boolean, ready, volume

```yaml
when:
  - field: "{{ pvcBound .children.pvc }}"
    equals: "true"
```

---

### `pvcPhase`

Returns `status.phase` — `"Bound"`, `"Pending"`, `"Released"`, or `"Lost"`.

Keywords: pvc, storage, phase, status, string, lifecycle, volume

```yaml
- path: pvcPhase
  value: "{{ pvcPhase .children.pvc }}"
# → "Bound"
```

---

### `pvcCapacity`

Returns `spec.resources.requests.storage`.

Keywords: pvc, storage, capacity, size, string, volume, request

```yaml
- path: capacity
  value: "{{ pvcCapacity .children.pvc }}"
# → "10Gi"
```

---

### `pvcStorageClass`

Returns `spec.storageClassName`, or `""` when not set (cluster default applies).

Keywords: pvc, storage, class, string, provisioner, volume

```yaml
- path: storageClass
  value: "{{ pvcStorageClass .children.pvc }}"
# → "standard"
```

---

### `pvcAccessModes`

Returns a comma-separated list of `spec.accessModes`.

Keywords: pvc, storage, access, modes, list, string, readwriteonce, readonlymany

```yaml
- path: accessModes
  value: "{{ pvcAccessModes .children.pvc }}"
# → "ReadWriteOnce"
```

---

### `pvcProvisioner`

Returns the provisioner annotation from the bound PV (`pv.kubernetes.io/provisioned-by`). Requires `enrich: [pvc]`.

Keywords: pvc, storage, pv, provisioner, enriched, string, csi, ebs

```yaml
- path: provisioner
  value: "{{ pvcProvisioner .children.pvc }}"
# → "ebs.csi.aws.com"
```

---

### `pvcVolumeMode`

Returns the `volumeMode` of the bound PV — `"Filesystem"` or `"Block"`. Requires `enrich: [pvc]`.

Keywords: pvc, storage, pv, volume, mode, enriched, string, filesystem, block

```yaml
- path: volumeMode
  value: "{{ pvcVolumeMode .children.pvc }}"
# → "Filesystem"
```

---

### `pvcStorageClassProvisioner`

Returns `_storageClass.provisioner` — the provisioner declared on the StorageClass itself. Requires `enrich: [storageclass]`.

Keywords: pvc, storage, class, provisioner, enriched, string, csi

```yaml
- path: scProvisioner
  value: "{{ pvcStorageClassProvisioner .children.pvc }}"
# → "ebs.csi.aws.com"
```

---

### `pvcStorageClassReclaimPolicy`

Returns `_storageClass.spec.reclaimPolicy` — `"Delete"` or `"Retain"`. Requires `enrich: [storageclass]`.

Keywords: pvc, storage, class, reclaim, policy, enriched, string, delete, retain

```yaml
- path: reclaimPolicy
  value: "{{ pvcStorageClassReclaimPolicy .children.pvc }}"
# → "Delete"
```

---

## Quick reference

| Note | Signature | Returns | Enrichment |
|------|-----------|---------|------------|
| `pvcBound` | `(obj any)` | `bool` | none |
| `pvcPhase` | `(obj any)` | `string` | none |
| `pvcCapacity` | `(obj any)` | `string` | none |
| `pvcStorageClass` | `(obj any)` | `string` | none |
| `pvcAccessModes` | `(obj any)` | `string` | none |
| `pvcProvisioner` | `(obj any)` | `string` | `enrich: [pvc]` |
| `pvcVolumeMode` | `(obj any)` | `string` | `enrich: [pvc]` |
| `pvcStorageClassProvisioner` | `(obj any)` | `string` | `enrich: [storageclass]` |
| `pvcStorageClassReclaimPolicy` | `(obj any)` | `string` | `enrich: [storageclass]` |

---

**Next →** [22 — Node Notes](22-node.md)
