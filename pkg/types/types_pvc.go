// pkg/types/types_pvc.go
package types

// PVCTemplateSource declares one PersistentVolumeClaim to be managed by Inrun.
//
// Example:
//
//	onCreate:
//	  persistentVolumeClaims:
//	    - name: "{{ .metadata.name }}-data"
//	      storage: 10Gi
//	      accessModes: ["ReadWriteOnce"]
type PVCTemplateSource struct {
	// Name — PVC name. Required.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Namespace — target namespace. Default: CR namespace.
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`

	// StorageClassName — storage class to use. Empty means cluster default.
	StorageClassName string `yaml:"storageClassName,omitempty" json:"storageClassName,omitempty"`

	// AccessModes — access modes. Default: ["ReadWriteOnce"].
	// Supports: ReadWriteOnce, ReadOnlyMany, ReadWriteMany, ReadWriteOncePod.
	AccessModes []string `yaml:"accessModes,omitempty" json:"accessModes,omitempty"`

	// Storage — requested storage size (e.g. "10Gi"). Required.
	Storage string `yaml:"storage,omitempty" json:"storage,omitempty" validate:"required"`

	// VolumeMode — Filesystem or Block. Default: Filesystem.
	VolumeMode string `yaml:"volumeMode,omitempty" json:"volumeMode,omitempty"`

	// VolumeName — bind to a specific PV by name.
	VolumeName string `yaml:"volumeName,omitempty" json:"volumeName,omitempty"`

	// Labels — applied to PVC metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}

// PVTemplateSource declares one PersistentVolume to be managed by Inrun.
// PersistentVolumes are cluster-scoped — Namespace is ignored.
//
// Example (local/dev, HostPath):
//
//	onCreate:
//	  persistentVolumes:
//	    - name: "{{ .metadata.name }}-pv"
//	      capacity: 10Gi
//	      hostPath: /mnt/data
//
// Example (cloud, CSI):
//
//	onCreate:
//	  persistentVolumes:
//	    - name: "{{ .metadata.name }}-pv"
//	      capacity: 10Gi
//	      csiDriver: ebs.csi.aws.com
//	      csiVolumeHandle: "{{ .spec.volumeId }}"
type PVTemplateSource struct {
	// Name — PV name. Required.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// StorageClassName — storage class name.
	StorageClassName string `yaml:"storageClassName,omitempty" json:"storageClassName,omitempty"`

	// Capacity — storage capacity (e.g. "10Gi"). Required.
	Capacity string `yaml:"capacity,omitempty" json:"capacity,omitempty" validate:"required"`

	// AccessModes — access modes. Default: ["ReadWriteOnce"].
	AccessModes []string `yaml:"accessModes,omitempty" json:"accessModes,omitempty"`

	// ReclaimPolicy — Retain, Delete, or Recycle. Default: Retain.
	ReclaimPolicy string `yaml:"reclaimPolicy,omitempty" json:"reclaimPolicy,omitempty"`

	// HostPath — host path for HostPath volume type. Used for local/dev PVs.
	// Mutually exclusive with csiDriver/csiVolumeHandle.
	HostPath string `yaml:"hostPath,omitempty" json:"hostPath,omitempty"`

	// CSIDriver — the CSI driver name for cloud-backed volumes
	// (e.g. "ebs.csi.aws.com", "pd.csi.storage.gke.io"). Declare alongside
	// csiVolumeHandle; mutually exclusive with hostPath.
	CSIDriver string `yaml:"csiDriver,omitempty" json:"csiDriver,omitempty"`

	// CSIVolumeHandle — the underlying storage system's unique volume
	// identifier (e.g. an EBS volume ID), used together with csiDriver to
	// bind this PV to an existing cloud volume.
	CSIVolumeHandle string `yaml:"csiVolumeHandle,omitempty" json:"csiVolumeHandle,omitempty"`

	// Labels — applied to PV metadata. Values support template expressions.
	Labels Labels `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Reconcile: true — also apply this declaration as drift correction on every
	// reconcile. Equivalent to declaring the same entry under both onCreate and
	// onReconcile. When false (default), only runs on onCreate (idempotent create).
	Reconcile bool `yaml:"reconcile,omitempty" json:"reconcile,omitempty"`

	// When lists conditions that must all pass for this entry to be written; see
	// Condition. When an entry kept in sync (onReconcile, or reconcile: true)
	// stops passing, the object it wrote is removed.
	When []Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// Or lists conditions of which at least one must pass, alongside When.
	Or []Condition `yaml:"or,omitempty" json:"or,omitempty"`

	// ForEach declares dynamic expansion over a list field.
	// When set, one source declaration becomes N declarations — one per list element.
	// .item and .<as> are available in template expressions within this declaration.
	//
	//	forEach:
	//	  field: spec.regions
	//	  as: region
	ForEach *ForEachSpec `yaml:"forEach,omitempty" json:"forEach,omitempty"`

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool `yaml:"forceConflict,omitempty" json:"forceConflict,omitempty"`
}
