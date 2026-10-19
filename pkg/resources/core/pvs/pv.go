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

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
)

// Create creates a PersistentVolume if it does not already exist.
// PVs are cluster-scoped — owner references are set as labels only.
func Create(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}
	_, err := kube.Clientset().CoreV1().PersistentVolumes().Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("pv.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().Str("pv", spec.Name).Msg("pv already exists — skipping create")
		return nil
	}

	pv := Build(owner, spec)
	_, err = kube.Clientset().CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pv.Create: creating %q: %w", spec.Name, err)
	}

	logger.Info().Str("pv", spec.Name).Str("owner", owner.GetName()).Msg("pv created")
	return nil
}

// Apply creates or updates a PersistentVolume using Server-Side Apply.
// PVs are cluster-scoped — no namespace arg. Sends only fields Inrun owns.
func Apply(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	pv := Build(owner, spec)
	pv.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolume"}

	body, err := json.Marshal(pv)
	if err != nil {
		return fmt.Errorf("pv.Apply: marshal: %w", err)
	}

	if _, err = kube.Clientset().CoreV1().PersistentVolumes().Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		metav1.PatchOptions{FieldManager: config.FieldManagerRuntime, Force: shared.ResolveForceConflict(kube, spec.ForceConflict)},
	); err != nil {
		return fmt.Errorf("pv.Apply: %w", err)
	}

	logger.Debug().
		Str("pv", spec.Name).
		Str("owner", owner.GetName()).
		Msg("pv applied")

	return nil
}

// Delete deletes the PV if it exists.
func Delete(ctx context.Context, kube kubeclient.Interface, name string) error {
	err := kube.Clientset().CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("pv.Delete: %w", err)
	}
	logger.Info().Str("pv", name).Msg("pv deleted")
	return nil
}

// DeleteIfOwned deletes the PV only if the owner label matches.
func DeleteIfOwned(ctx context.Context, kube kubeclient.Interface, owner domain.Object, name string) error {
	existing, err := kube.Clientset().CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.Clientset().CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the PersistentVolume spec describes, owned by owner.
// It reads nothing from the cluster.
func Build(owner domain.Object, spec Spec) *corev1.PersistentVolume {
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
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

	pv.OwnerReferences = shared.ResolveOwnerReferences(owner)

	return pv
}
