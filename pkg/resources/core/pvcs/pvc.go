// pkg/resources/pvcs/pvc.go
package pvcs

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Create creates a PVC owned by the CR if it does not already exist.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("pvc.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().Str("pvc", spec.Name).Str("namespace", namespace).Msg("pvc already exists — skipping create")
		return nil
	}

	pvc := Build(owner, spec)
	_, err = kube.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pvc.Create: creating %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().Str("pvc", spec.Name).Str("namespace", namespace).Str("owner", owner.GetName()).Msg("pvc created")
	return nil
}

// Apply reconciles a PVC. PVC spec is largely immutable after creation, so it
// reads the PVC and updates only its labels on drift.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	existing, err := kube.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return Create(ctx, kube, owner, spec)
		}
		return fmt.Errorf("pvc.Apply: getting %q: %w", spec.Name, err)
	}

	updated := existing.DeepCopy()
	drifted := false

	for k, v := range spec.Labels {
		if updated.Labels[k] != v {
			updated.Labels[k] = v
			drifted = true
		}
	}

	if !drifted {
		logger.Debug().Str("pvc", spec.Name).Msg("pvc in sync — no update needed")
		return nil
	}

	_, err = kube.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("pvc.Apply: updating %q: %w", spec.Name, err)
	}

	logger.Info().Str("pvc", spec.Name).Str("namespace", namespace).Msg("pvc updated")
	return nil
}

// Delete deletes the PVC if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("pvc.Delete: %w", err)
	}
	logger.Info().Str("pvc", spec.Name).Str("owner", owner.GetName()).Msg("pvc deleted")
	return nil
}

// DeleteIfOwned deletes the PVC only if it is owned by the given CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, owner types.Owner, name, namespace string) error {
	existing, err := kube.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the PersistentVolumeClaim spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.PersistentVolumeClaim {
	ns := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())

	storageQty := resource.MustParse(spec.Storage)

	var accessModes []corev1.PersistentVolumeAccessMode
	for _, m := range spec.AccessModes {
		accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(m))
	}

	volumeMode := corev1.PersistentVolumeFilesystem
	if spec.VolumeMode == "Block" {
		volumeMode = corev1.PersistentVolumeBlock
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       ns,
			Labels:          spec.Labels,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			VolumeMode:  &volumeMode,
			VolumeName:  spec.VolumeName,
			// DataSourceRef: ,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQty,
				},
			},
		},
	}

	if spec.StorageClassName != "" {
		pvc.Spec.StorageClassName = &spec.StorageClassName
	}

	return pvc
}
