package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/kubewright/core/pvcs"
	"github.com/intentd/kubewright/core/pvs"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ResolvePVC resolves all template expressions in a PVCTemplateSource.
func ResolvePVC(r *template.Resolver, src types.PVCTemplateSource) (types.PVCTemplateSource, error) {
	resolved := types.PVCTemplateSource{
		AccessModes: src.AccessModes,
		VolumeMode:  src.VolumeMode,
	}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("pvc.name: %w", err)
	}
	if resolved.StorageClassName, err = r.Resolve(src.StorageClassName); err != nil {
		return resolved, fmt.Errorf("pvc.storageClassName: %w", err)
	}
	if resolved.Storage, err = r.Resolve(src.Storage); err != nil {
		return resolved, fmt.Errorf("pvc.storage: %w", err)
	}
	if resolved.VolumeName, err = r.Resolve(src.VolumeName); err != nil {
		return resolved, fmt.Errorf("pvc.volumeName: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("pvc.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("pvc.labels: %w", err)
	}

	return resolved, nil
}

// ResolvePV resolves all template expressions in a PVTemplateSource.
func ResolvePV(r *template.Resolver, src types.PVTemplateSource) (types.PVTemplateSource, error) {
	resolved := types.PVTemplateSource{
		AccessModes: src.AccessModes,
	}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("pv.name: %w", err)
	}
	if resolved.StorageClassName, err = r.Resolve(src.StorageClassName); err != nil {
		return resolved, fmt.Errorf("pv.storageClassName: %w", err)
	}
	if resolved.Capacity, err = r.Resolve(src.Capacity); err != nil {
		return resolved, fmt.Errorf("pv.capacity: %w", err)
	}
	if resolved.ReclaimPolicy, err = r.Resolve(src.ReclaimPolicy); err != nil {
		return resolved, fmt.Errorf("pv.reclaimPolicy: %w", err)
	}
	if resolved.HostPath, err = r.Resolve(src.HostPath); err != nil {
		return resolved, fmt.Errorf("pv.hostPath: %w", err)
	}
	if resolved.CSIDriver, err = r.Resolve(src.CSIDriver); err != nil {
		return resolved, fmt.Errorf("pv.csiDriver: %w", err)
	}
	if resolved.CSIVolumeHandle, err = r.Resolve(src.CSIVolumeHandle); err != nil {
		return resolved, fmt.Errorf("pv.csiVolumeHandle: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("pv.labels: %w", err)
	}

	return resolved, nil
}

// PVCSpec builds a pvcs.Spec from a PVCTemplateSource.
func PVCSpec(src types.PVCTemplateSource, owner domain.Object) pvcs.Spec {
	spec := pvcs.Spec{
		Name:             src.Name,
		Namespace:        src.Namespace,
		StorageClassName: src.StorageClassName,
		AccessModes:      src.AccessModes,
		Storage:          src.Storage,
		VolumeMode:       src.VolumeMode,
		VolumeName:       src.VolumeName,
		Labels:           make(map[string]string),
	}

	if len(spec.AccessModes) == 0 {
		spec.AccessModes = []string{"ReadWriteOnce"}
	}
	if spec.VolumeMode == "" {
		spec.VolumeMode = "Filesystem"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// PVSpec builds a pvs.Spec from a PVTemplateSource.
func PVSpec(src types.PVTemplateSource, owner domain.Object) pvs.Spec {
	spec := pvs.Spec{
		Name:             src.Name,
		StorageClassName: src.StorageClassName,
		Capacity:         src.Capacity,
		AccessModes:      src.AccessModes,
		ReclaimPolicy:    src.ReclaimPolicy,
		HostPath:         src.HostPath,
		CSIDriver:        src.CSIDriver,
		CSIVolumeHandle:  src.CSIVolumeHandle,
		Labels:           make(map[string]string),
	}

	if len(spec.AccessModes) == 0 {
		spec.AccessModes = []string{"ReadWriteOnce"}
	}
	if spec.ReclaimPolicy == "" {
		spec.ReclaimPolicy = "Retain"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}
