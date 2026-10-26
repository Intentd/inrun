# PersistentVolume

This declares one PersistentVolume to be managed by Inrun. PersistentVolumes are cluster-scoped — Namespace is ignored.

Example (local/dev, HostPath):

```yaml
onCreate:
  persistentVolumes:
    - name: "{{ .metadata.name }}-pv"
      capacity: 10Gi
      hostPath: /mnt/data
```

Example (cloud, CSI):

```yaml
onCreate:
  persistentVolumes:
    - name: "{{ .metadata.name }}-pv"
      capacity: 10Gi
      csiDriver: ebs.csi.aws.com
      csiVolumeHandle: "{{ .spec.volumeId }}"
```

---

## Lifecycle

Declare this resource under `onCreate` for an idempotent, one-time create: Inrun creates it on the first reconcile and leaves it untouched afterward. Set `reconcile: true` on the same entry to also apply it as drift correction on every subsequent reconcile. This is a shorthand for declaring the identical entry under `onReconcile` as well — there's no need to do both.

Declare a resource under `onDelete` to run explicit cleanup before the CR's finalizer is removed. Most resources need no `onDelete` entry — they are garbage-collected automatically through owner references when the CR itself is deleted.

---

## Fields

### `name`

Type: string

Name — PV name. Required.

---

### `storageClassName`

Type: string

StorageClassName — storage class name.

---

### `capacity`

Type: string

Capacity — storage capacity (e.g. "10Gi"). Required.

---

### `accessModes`

Type: list

AccessModes — access modes. Default: \["ReadWriteOnce"].

---

### `reclaimPolicy`

Type: string

ReclaimPolicy — Retain, Delete, or Recycle. Default: Retain.

---

### `hostPath`

Type: string

HostPath — host path for HostPath volume type. Used for local/dev PVs. Mutually exclusive with csiDriver/csiVolumeHandle.

---

### `csiDriver`

Type: string

CSIDriver — the CSI driver name for cloud-backed volumes (e.g. "ebs.csi.aws.com", "pd.csi.storage.gke.io"). Declare alongside csiVolumeHandle; mutually exclusive with hostPath.

---

### `csiVolumeHandle`

Type: string

CSIVolumeHandle — the underlying storage system's unique volume identifier (e.g. an EBS volume ID), used together with csiDriver to bind this PV to an existing cloud volume.

---

### `labels`

Type: map

Labels — applied to PV metadata. Values support template expressions.

---

### `reconcile`

Type: boolean

Reconcile: true — also apply this declaration as drift correction on every reconcile. Equivalent to declaring the same entry under both onCreate and onReconcile. When false (default), only runs on onCreate (idempotent create).

---

### `when`

Type: list

When lists conditions that must all pass for this entry to be written; see Condition. When an entry kept in sync (onReconcile, or reconcile: true) stops passing, the object it wrote is removed.

---

### `or`

Type: list

Or lists conditions of which at least one must pass, alongside When.

---

### `forEach`

Type: object

ForEach declares dynamic expansion over a list field. When set, one source declaration becomes N declarations — one per list element. .item and .\<as> are available in template expressions within this declaration.

```yaml
forEach:
  field: spec.regions
  as: region
```

---

### `forceConflict`

Type: boolean

ForceConflict, when true, sets Force: true when applying this resource, taking ownership of conflicting fields instead of returning a conflict error. Overrides the CRD-level ForceConflict setting.

---

## Quick reference

| YAML key | Type |
|---|---|
| `name` | string |
| `storageClassName` | string |
| `capacity` | string |
| `accessModes` | list |
| `reclaimPolicy` | string |
| `hostPath` | string |
| `csiDriver` | string |
| `csiVolumeHandle` | string |
| `labels` | map |
| `reconcile` | boolean |
| `when` | list |
| `or` | list |
| `forEach` | object |
| `forceConflict` | boolean |
