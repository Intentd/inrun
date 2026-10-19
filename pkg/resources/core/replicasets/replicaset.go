// pkg/resources/replicasets/replicaset.go
package replicasets

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Create creates a ReplicaSet owned by the CR if it does not already exist.
// Idempotent — if the ReplicaSet exists, does nothing and returns nil.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("replicaset.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.AppsV1().ReplicaSets(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("replicaset.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	rs := Build(owner, spec)

	_, err = kube.AppsV1().ReplicaSets(namespace).Create(ctx, rs, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("replicaset.Create: creating replicaset %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates a ReplicaSet using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("replicaset.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	rs := Build(owner, spec)
	rs.TypeMeta = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"}

	body, err := json.Marshal(rs)
	if err != nil {
		return "", fmt.Errorf("replicaset.Apply: marshal: %w", err)
	}

	if _, err = kube.AppsV1().ReplicaSets(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("replicaset.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the ReplicaSet if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.AppsV1().ReplicaSets(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("replicaset.Delete: deleting replicaset %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the ReplicaSet only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.AppsV1().ReplicaSets(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", err
	}

	if !owned(existing) {
		return types.NotOwned, nil
	}

	if err := kube.AppsV1().ReplicaSets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the ReplicaSet spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *appsv1.ReplicaSet {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	replicas := spec.Replicas
	var pullSecrets []corev1.LocalObjectReference
	for _, name := range spec.ImagePullSecrets {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{
			Name: name,
		})
	}

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			Annotations:     spec.Annotations,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: shared.PodSelector(spec.Selector, spec.Name),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: shared.PodLabels(spec.Labels, shared.PodSelector(spec.Selector, spec.Name)),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets:   pullSecrets,
					ServiceAccountName: spec.ServiceAccountName,
					NodeSelector:       spec.NodeSelector,
					Containers: []corev1.Container{
						{
							Name:  spec.Name,
							Image: spec.Image,
						},
					},
				},
			},
		},
	}

	if spec.Port > 0 {
		rs.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: spec.Port, Protocol: spec.Protocol},
		}
	}

	if spec.Resources != nil {
		rs.Spec.Template.Spec.Containers[0].Resources = shared.BuildResourceRequirements(spec.Resources)
	}

	shared.ApplyProbes(&rs.Spec.Template.Spec.Containers[0], spec.Probes, spec.Port, spec.Presets)

	// Security
	shared.ApplySecurityContext(&rs.Spec.Template.Spec.Containers[0], &rs.Spec.Template.Spec, spec.SecurityContext, spec.PodSecurity)

	if len(spec.Env) > 0 {
		rs.Spec.Template.Spec.Containers[0].Env = make([]corev1.EnvVar, 0, len(spec.Env))
		for _, ev := range spec.Env {
			kev := corev1.EnvVar{Name: ev.Name}
			if ev.ValueFrom != nil {
				kev.ValueFrom = &corev1.EnvVarSource{}
				if ev.ValueFrom.SecretKeyRef != nil {
					kev.ValueFrom.SecretKeyRef = &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: ev.ValueFrom.SecretKeyRef.Name},
						Key:                  ev.ValueFrom.SecretKeyRef.Key,
						Optional:             ev.ValueFrom.SecretKeyRef.Optional,
					}
				}
				if ev.ValueFrom.ConfigMapKeyRef != nil {
					kev.ValueFrom.ConfigMapKeyRef = &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: ev.ValueFrom.ConfigMapKeyRef.Name},
						Key:                  ev.ValueFrom.ConfigMapKeyRef.Key,
						Optional:             ev.ValueFrom.ConfigMapKeyRef.Optional,
					}
				}
			} else {
				kev.Value = ev.Value
			}
			rs.Spec.Template.Spec.Containers[0].Env = append(rs.Spec.Template.Spec.Containers[0].Env, kev)
		}
	}

	envFrom, extraEnv := shared.ExpandEnvFrom(spec.EnvFrom)
	rs.Spec.Template.Spec.Containers[0].EnvFrom = envFrom
	rs.Spec.Template.Spec.Containers[0].Env = append(rs.Spec.Template.Spec.Containers[0].Env, extraEnv...)

	// Volumes / VolumeMounts
	if vols := shared.BuildVolumes(spec.Volumes); len(vols) > 0 {
		rs.Spec.Template.Spec.Volumes = vols
	}
	if mounts := shared.BuildVolumeMounts(spec.VolumeMounts); len(mounts) > 0 {
		rs.Spec.Template.Spec.Containers[0].VolumeMounts = mounts
	}

	return rs
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
