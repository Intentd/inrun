// pkg/resources/serviceaccounts/serviceaccount.go
package serviceaccounts

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
)

// Spec is the fully resolved ServiceAccount specification.
type Spec struct {
	// Name — ServiceAccount name. Required.
	Name string

	// Namespace — target namespace. Required.
	Namespace string

	// Labels — applied to ServiceAccount metadata.
	Labels map[string]string

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a ServiceAccount if it does not already exist.
// Idempotent — skips if the ServiceAccount exists.
// Owner reference set so ServiceAccount is garbage collected when CR is deleted.
//
// ServiceAccounts have no meaningful spec fields that can drift after creation.
// There is no Apply function — Create is called from both onCreate and
// onReconcile paths. If it exists, it stays. If it was deleted, it is recreated.
func Create(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("serviceaccount.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.Clientset().CoreV1().ServiceAccounts(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("serviceaccount.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("serviceaccount", spec.Name).
			Str("namespace", namespace).
			Msg("serviceaccount already exists — skipping create")
		return nil
	}

	sa := Build(owner, spec)

	_, err = kube.Clientset().CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("serviceaccount.Create: creating serviceaccount %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("serviceaccount", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("serviceaccount created")

	return nil
}

// Delete deletes the ServiceAccount if it exists.
// For most cases owner references handle cleanup automatically —
// only use this when explicit cleanup control is needed.
func Delete(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.Clientset().CoreV1().ServiceAccounts(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("serviceaccount", spec.Name).
				Str("namespace", namespace).
				Msg("serviceaccount already deleted — skipping")
			return nil
		}
		return fmt.Errorf("serviceaccount.Delete: deleting serviceaccount %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("serviceaccount", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("serviceaccount deleted")

	return nil
}

// DeleteIfOwned deletes the ServiceAccount if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubeclient.Interface,
	owner domain.Object, name, namespace string) error {

	existing, err := kube.Clientset().CoreV1().ServiceAccounts(namespace).
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
	return kube.Clientset().CoreV1().ServiceAccounts(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the ServiceAccount spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner domain.Object, spec Spec) *corev1.ServiceAccount {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
	}
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
