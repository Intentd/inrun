# PVC

Read a PersistentVolumeClaim (whether it is bound, its phase, capacity, storage class and access modes), the PersistentVolume bound to it, and its StorageClass.

## Reference

| Function | Description |
|----------|-------------|
| [`pvProvisioner`](#pvprovisioner) | Returns the provisioner that created a PersistentVolume (annotation pv.kubernetes.io/provisioned-by). |
| [`pvVolumeMode`](#pvvolumemode) | Returns a PersistentVolume's volumeMode: Filesystem or Block. |
| [`pvcAccessModes`](#pvcaccessmodes) | Returns the PVC's spec.accessModes. |
| [`pvcBound`](#pvcbound) | Returns `true` when `status.phase == "Bound"`. |
| [`pvcCapacity`](#pvccapacity) | Returns `spec.resources.requests.storage`. |
| [`pvcPhase`](#pvcphase) | Returns `status.phase` — `"Bound"`, `"Pending"`, `"Released"`, or `"Lost"`. |
| [`pvcStorageClass`](#pvcstorageclass) | Returns `spec.storageClassName`, or `""` when not set (cluster default applies). |
| [`storageClassProvisioner`](#storageclassprovisioner) | Returns a StorageClass's provisioner. |
| [`storageClassReclaimPolicy`](#storageclassreclaimpolicy) | Returns a StorageClass's reclaimPolicy: Delete or Retain. |

## Functions

### pvProvisioner

`pvProvisioner` returns the provisioner that created a PersistentVolume (annotation pv.kubernetes.io/provisioned-by).

```text
{{ pvProvisioner .pv }}  → "ebs.csi.aws.com"
```

### pvVolumeMode

`pvVolumeMode` returns a PersistentVolume's volumeMode: Filesystem or Block.

```text
{{ pvVolumeMode .pv }}  → "Filesystem"
```

### pvcAccessModes

`pvcAccessModes` returns the PVC's spec.accessModes.

```text
{{ pvcAccessModes .pvc }}  → [ReadWriteOnce]
```

### pvcBound

`pvcBound` returns `true` when `status.phase == "Bound"`.

```text
{{ pvcBound .pvc }}  → true
```

### pvcCapacity

`pvcCapacity` returns `spec.resources.requests.storage`.

```text
{{ pvcCapacity .pvc }}  → "10Gi"
```

### pvcPhase

`pvcPhase` returns `status.phase` — `"Bound"`, `"Pending"`, `"Released"`, or `"Lost"`.

```text
{{ pvcPhase .pvc }}  → "Bound"
```

### pvcStorageClass

`pvcStorageClass` returns `spec.storageClassName`, or `""` when not set (cluster default applies).

```text
{{ pvcStorageClass .pvc }}  → "standard"
```

### storageClassProvisioner

`storageClassProvisioner` returns a StorageClass's provisioner.

```text
{{ storageClassProvisioner .storageClass }}  → "ebs.csi.aws.com"
```

### storageClassReclaimPolicy

`storageClassReclaimPolicy` returns a StorageClass's reclaimPolicy: Delete or Retain.

```text
{{ storageClassReclaimPolicy .storageClass }}  → "Delete"
```
