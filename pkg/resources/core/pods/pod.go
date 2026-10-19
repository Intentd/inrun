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
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Create creates a Pod owned by the CR if it does not already exist.
// Idempotent — if the Pod exists, does nothing and returns nil.
// Owner reference is set so the Pod is garbage collected when the CR is deleted.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("pod.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.CoreV1().Pods(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("pod.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	pod := Build(owner, spec)

	_, err = kube.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("pod.Create: creating pod %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates a Pod using Server-Side Apply.
// Pod specs are largely immutable — if SSA is rejected (422/Invalid),
// falls back to delete + recreate.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("pod.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	p := Build(owner, spec)
	p.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}

	body, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("pod.Apply: marshal: %w", err)
	}

	if _, err = kube.CoreV1().Pods(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		if errors.IsInvalid(err) {
			if delErr := kube.CoreV1().Pods(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
				return "", fmt.Errorf("pod.Apply: deleting stale pod %q: %w", spec.Name, delErr)
			}
			if _, err := Create(ctx, kube, owner, spec); err != nil {
				return "", err
			}
			return types.Replaced, nil
		}
		return "", fmt.Errorf("pod.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the Pod if it exists.
// For most cases owner references handle cascade deletion automatically —
// only use this when you need explicit cleanup control in onDelete.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.CoreV1().Pods(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("pod.Delete: deleting pod %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the Pod if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().Pods(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", err
	}
	// Only delete if we own it
	if !owned(existing) {
		return types.NotOwned, nil
	}
	if err := kube.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the Pod spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.Pod {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			Annotations:     spec.Annotations,
			OwnerReferences: types.OwnerReferences(owner),
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
