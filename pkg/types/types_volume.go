package types

import restypes "github.com/intentd/kuberesources/core/types"

// ── Volume / VolumeMount ──────────────────────────────────────────────────────
//
// VolumeSource and VolumeMount declare pod volumes and container mounts in the
// same way as Kubernetes, but with template expression support in name fields
// so volume names can be derived from the CR at reconcile time.
//
// Supported volume types: configMap, secret, emptyDir, persistentVolumeClaim,
// hostPath. Fields that accept template expressions are marked with (template).
//
// Example:
//
//	volumes:
//	  - name: app-config                          # (template)
//	    configMap:
//	      name: "{{ .metadata.name }}-config"     # (template)
//
//	  - name: creds
//	    secret:
//	      name: "{{ .metadata.name }}-creds"      # (template)
//
//	  - name: tmp
//	    emptyDir: {}
//
//	  - name: data
//	    persistentVolumeClaim:
//	      claimName: "{{ .metadata.name }}-pvc"   # (template)
//
//	  - name: host-logs
//	    hostPath:
//	      path: /var/log
//
//	volumeMounts:
//	  - name: app-config
//	    mountPath: /etc/config
//	    subPath: app.json
//	    readOnly: true

// VolumeSource is restypes.VolumeSource.
type VolumeSource = restypes.VolumeSource

// ConfigMapVolumeSource is restypes.ConfigMapVolumeSource.
type ConfigMapVolumeSource = restypes.ConfigMapVolumeSource

// SecretVolumeSource is restypes.SecretVolumeSource.
type SecretVolumeSource = restypes.SecretVolumeSource

// EmptyDirVolumeSource is restypes.EmptyDirVolumeSource.
type EmptyDirVolumeSource = restypes.EmptyDirVolumeSource

// PVCVolumeSource is restypes.PVCVolumeSource.
type PVCVolumeSource = restypes.PVCVolumeSource

// HostPathVolumeSource is restypes.HostPathVolumeSource.
type HostPathVolumeSource = restypes.HostPathVolumeSource

// VolumeMount is restypes.VolumeMount.
type VolumeMount = restypes.VolumeMount
