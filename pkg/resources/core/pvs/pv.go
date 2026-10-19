// pkg/resources/pvs/pv.go
package pvs

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Create creates a PersistentVolume if it does not already exist.
// PVs are cluster-scoped — owner references are set as labels only.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	_, err := kube.CoreV1().PersistentVolumes().Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("pv.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	pv := Build(owner, spec)
	_, err = kube.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("pv.Create: creating %q: %w", spec.Name, err)
	}

	return types.Created, nil
}

// Apply creates or updates a PersistentVolume using Server-Side Apply.
// PVs are cluster-scoped — no namespace arg. Sends only fields Inrun owns.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {

	pv := Build(owner, spec)
	pv.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolume"}

	body, err := json.Marshal(pv)
	if err != nil {
		return "", fmt.Errorf("pv.Apply: marshal: %w", err)
	}

	if _, err = kube.CoreV1().PersistentVolumes().Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("pv.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the PV if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, name string) (types.Result, error) {
	err := kube.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("pv.Delete: %w", err)
	}
	return types.Deleted, nil
}

// DeleteIfOwned deletes the PV only if the owner label matches.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", err
	}
	if !owned(existing) {
		return types.NotOwned, nil
	}
	if err := kube.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the PersistentVolume spec describes, owned by owner.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.PersistentVolume {
	capacityQty := resource.MustParse(spec.Capacity)

	var accessModes []corev1.PersistentVolumeAccessMode
	for _, m := range spec.AccessModes {
		accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(m))
	}

	reclaimPolicy := corev1.PersistentVolumeReclaimRetain
	switch spec.ReclaimPolicy {
	case "Delete":
		reclaimPolicy = corev1.PersistentVolumeReclaimDelete
	case "Recycle":
		reclaimPolicy = corev1.PersistentVolumeReclaimRecycle
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   spec.Name,
			Labels: spec.Labels,
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: capacityQty,
			},
			AccessModes:                   accessModes,
			PersistentVolumeReclaimPolicy: reclaimPolicy,
			StorageClassName:              spec.StorageClassName,
		},
	}

	if spec.HostPath != "" {
		pv.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: spec.HostPath},
		}
	} else if spec.CSIDriver != "" {
		pv.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       spec.CSIDriver,
				VolumeHandle: spec.CSIVolumeHandle,
			},
		}
	}

	pv.OwnerReferences = types.OwnerReferences(owner)

	return pv
}
