// pkg/resources/pods/pod.go
package pods

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
)

// Create creates a Pod owned by the CR if it does not already exist.
// Idempotent — if the Pod exists, does nothing and returns nil.
// Owner reference is set so the Pod is garbage collected when the CR is deleted.
func Create(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("pod.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.Clientset().CoreV1().Pods(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("pod.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("pod", spec.Name).
			Str("namespace", namespace).
			Msg("pod already exists — skipping create")
		return nil
	}

	pod := Build(owner, spec)

	_, err = kube.Clientset().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pod.Create: creating pod %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("pod", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("pod created")

	return nil
}

// Apply creates or updates a Pod using Server-Side Apply.
// Pod specs are largely immutable — if SSA is rejected (422/Invalid),
// falls back to delete + recreate.
func Apply(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("pod.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	p := Build(owner, spec)
	p.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}

	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("pod.Apply: marshal: %w", err)
	}

	if _, err = kube.Clientset().CoreV1().Pods(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		metav1.PatchOptions{FieldManager: config.FieldManagerRuntime, Force: shared.ResolveForceConflict(kube, spec.ForceConflict)},
	); err != nil {
		if errors.IsInvalid(err) {
			logger.Info().Str("pod", spec.Name).Msg("pod spec immutable — delete+recreate")
			if delErr := kube.Clientset().CoreV1().Pods(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
				return fmt.Errorf("pod.Apply: deleting stale pod %q: %w", spec.Name, delErr)
			}
			return Create(ctx, kube, owner, spec)
		}
		return fmt.Errorf("pod.Apply: %w", err)
	}

	logger.Debug().
		Str("pod", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("pod applied")

	return nil
}

// Delete deletes the Pod if it exists.
// For most cases owner references handle cascade deletion automatically —
// only use this when you need explicit cleanup control in onDelete.
func Delete(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.Clientset().CoreV1().Pods(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("pod", spec.Name).
				Str("namespace", namespace).
				Msg("pod already deleted — skipping")
			return nil
		}
		return fmt.Errorf("pod.Delete: deleting pod %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("pod", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("pod deleted")

	return nil
}

// DeleteIfOwned deletes the Pod if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubeclient.Interface,
	owner domain.Object, name, namespace string) error {

	existing, err := kube.Clientset().CoreV1().Pods(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// Only delete if we own it
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.Clientset().CoreV1().Pods(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the Pod spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner domain.Object, spec Spec) *corev1.Pod {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			Annotations:     spec.Annotations,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
		Spec: corev1.PodSpec{
			ImagePullSecrets:   shared.ToPullSecrets(spec.ImagePullSecrets),
			ServiceAccountName: spec.ServiceAccountName,
			NodeSelector:       spec.NodeSelector,
			Containers: []corev1.Container{
				{
					Name:  spec.Name,
					Image: spec.Image,
				},
			},
		},
	}

	if spec.Port > 0 {
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: int32(spec.Port), Protocol: spec.Protocol},
		}
	}

	if spec.Resources != nil {
		pod.Spec.Containers[0].Resources = shared.BuildResourceRequirements(spec.Resources)
	}

	shared.ApplyProbes(&pod.Spec.Containers[0], spec.Probes, int32(spec.Port), spec.Presets)

	// Security
	shared.ApplySecurityContext(&pod.Spec.Containers[0], &pod.Spec, spec.SecurityContext, spec.PodSecurity)

	// Volumes / VolumeMounts
	if vols := shared.BuildVolumes(spec.Volumes); len(vols) > 0 {
		pod.Spec.Volumes = vols
	}
	if mounts := shared.BuildVolumeMounts(spec.VolumeMounts); len(mounts) > 0 {
		pod.Spec.Containers[0].VolumeMounts = mounts
	}

	return pod
}

func validateSpec(spec Spec) error {
	var missing []string
	if spec.Name == "" {
		missing = append(missing, "name")
	}
	if spec.Image == "" {
		missing = append(missing, "image")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %v", missing)
	}
	return nil
}
