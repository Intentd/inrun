// pkg/resources/rolebindings/rolebinding.go
package rolebindings

import (
	"context"
	"encoding/json"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/shared"
	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Spec is the fully resolved RoleBinding specification.
type Spec struct {
	Name      string
	Namespace string
	Labels    map[string]string
	RoleRef   rbacv1.RoleRef
	Subjects  []rbacv1.Subject

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a RoleBinding if it does not already exist.
// Idempotent — skips if the RoleBinding already exists.
// Owner reference ensures cleanup when the CR is deleted.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("rolebinding.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.RbacV1().RoleBindings(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("rolebinding.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("rolebinding", spec.Name).
			Str("namespace", namespace).
			Msg("rolebinding already exists — skipping create")
		return nil
	}

	rb := Build(owner, spec)

	_, err = kube.RbacV1().RoleBindings(namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("rolebinding.Create: creating rolebinding %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("rolebinding", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("rolebinding created")

	return nil
}

// Apply creates or updates a RoleBinding using Server-Side Apply.
// RoleRef is immutable — if SSA is rejected due to a changed roleRef,
// the binding is deleted and recreated.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	rb := Build(owner, spec)
	rb.TypeMeta = metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding"}

	body, err := json.Marshal(rb)
	if err != nil {
		return fmt.Errorf("rolebinding.Apply: marshal: %w", err)
	}

	if _, err = kube.RbacV1().RoleBindings(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		if errors.IsInvalid(err) {
			// roleRef is immutable — delete and recreate.
			logger.Info().Str("rolebinding", spec.Name).Msg("rolebinding roleRef drifted — delete+recreate")
			if delErr := kube.RbacV1().RoleBindings(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
				return fmt.Errorf("rolebinding.Apply: deleting stale binding %q: %w", spec.Name, delErr)
			}
			return Create(ctx, kube, owner, spec)
		}
		return fmt.Errorf("rolebinding.Apply: %w", err)
	}

	logger.Debug().
		Str("rolebinding", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("rolebinding applied")

	return nil
}

// Delete deletes the RoleBinding if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.RbacV1().RoleBindings(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("rolebinding", spec.Name).
				Str("namespace", namespace).
				Msg("rolebinding already deleted — skipping")
			return nil
		}
		return fmt.Errorf("rolebinding.Delete: deleting rolebinding %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("rolebinding", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("rolebinding deleted")

	return nil
}

// DeleteIfOwned deletes the RoleBinding only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface,
	owner types.Owner, name, namespace string) error {

	existing, err := kube.RbacV1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.RbacV1().RoleBindings(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the RoleBinding spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *rbacv1.RoleBinding {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
		RoleRef:  spec.RoleRef,
		Subjects: spec.Subjects,
	}
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
