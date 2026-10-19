// pkg/resources/serviceaccounts/serviceaccount.go
package serviceaccounts

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Spec is the fully resolved ServiceAccount specification.
type Spec struct {
	// Name — ServiceAccount name. Required.
	Name string

	// Namespace — target namespace. Required.
	Namespace string

	// Labels — applied to ServiceAccount metadata.
	Labels map[string]string
}

// Create creates a ServiceAccount if it does not already exist.
// Idempotent — skips if the ServiceAccount exists.
// Owner reference set so ServiceAccount is garbage collected when CR is deleted.
//
// ServiceAccounts have no meaningful spec fields that can drift after creation.
// There is no Apply function — Create is called from both onCreate and
// onReconcile paths. If it exists, it stays. If it was deleted, it is recreated.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("serviceaccount.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.CoreV1().ServiceAccounts(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("serviceaccount.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	sa := Build(owner, spec)

	_, err = kube.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("serviceaccount.Create: creating serviceaccount %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Delete deletes the ServiceAccount if it exists.
// For most cases owner references handle cleanup automatically —
// only use this when explicit cleanup control is needed.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.CoreV1().ServiceAccounts(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("serviceaccount.Delete: deleting serviceaccount %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the ServiceAccount if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().ServiceAccounts(namespace).
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
	if err := kube.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the ServiceAccount spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.ServiceAccount {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: types.OwnerReferences(owner),
		},
	}
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
