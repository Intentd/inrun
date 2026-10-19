// pkg/resources/clusterrolebindings/clusterrolebinding.go
package clusterrolebindings

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

// Spec is the fully resolved ClusterRoleBinding specification.
type Spec struct {
	Name     string
	Labels   map[string]string
	RoleRef  rbacv1.RoleRef
	Subjects []rbacv1.Subject
	Sleep    string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a ClusterRoleBinding if it does not already exist.
// Idempotent — skips if the ClusterRoleBinding already exists.
// ClusterRoleBindings are cluster-scoped; ownership is tracked via the inrun.dev/owner label.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("clusterrolebinding.Create: invalid spec: %w", err)
	}
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.RbacV1().ClusterRoleBindings().Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("clusterrolebinding.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("clusterrolebinding", spec.Name).
			Msg("clusterrolebinding already exists — skipping create")
		return nil
	}

	crb := Build(owner, spec)

	_, err = kube.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("clusterrolebinding.Create: creating %q: %w", spec.Name, err)
	}

	logger.Info().
		Str("clusterrolebinding", spec.Name).
		Str("owner", owner.GetName()).
		Msg("clusterrolebinding created")

	return nil
}

// Apply creates or updates a ClusterRoleBinding using Server-Side Apply.
// RoleRef is immutable — if SSA is rejected due to a changed roleRef,
// the binding is deleted and recreated.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) error {
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	crb := Build(owner, spec)
	crb.TypeMeta = metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"}

	body, err := json.Marshal(crb)
	if err != nil {
		return fmt.Errorf("clusterrolebinding.Apply: marshal: %w", err)
	}

	if _, err = kube.RbacV1().ClusterRoleBindings().Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		if errors.IsInvalid(err) {
			// roleRef is immutable — delete and recreate.
			logger.Info().Str("clusterrolebinding", spec.Name).Msg("clusterrolebinding roleRef drifted — delete+recreate")
			if delErr := kube.RbacV1().ClusterRoleBindings().Delete(ctx, spec.Name, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
				return fmt.Errorf("clusterrolebinding.Apply: deleting stale binding %q: %w", spec.Name, delErr)
			}
			return Create(ctx, kube, owner, spec)
		}
		return fmt.Errorf("clusterrolebinding.Apply: %w", err)
	}

	logger.Debug().
		Str("clusterrolebinding", spec.Name).
		Str("owner", owner.GetName()).
		Msg("clusterrolebinding applied")

	return nil
}

// Delete deletes the ClusterRoleBinding if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.RbacV1().ClusterRoleBindings().Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("clusterrolebinding.Delete: deleting %q: %w", spec.Name, err)
	}

	logger.Info().
		Str("clusterrolebinding", spec.Name).
		Str("owner", owner.GetName()).
		Msg("clusterrolebinding deleted")

	return nil
}

// DeleteIfOwned deletes the ClusterRoleBinding only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface,
	owner types.Owner, name string) error {

	existing, err := kube.RbacV1().ClusterRoleBindings().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.RbacV1().ClusterRoleBindings().Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the ClusterRoleBinding spec describes, owned by owner.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *rbacv1.ClusterRoleBinding {
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   spec.Name,
			Labels: spec.Labels,
		},
		RoleRef:  spec.RoleRef,
		Subjects: spec.Subjects,
	}
	if owner.GetNamespace() == "" {
		crb.OwnerReferences = shared.ResolveOwnerReferences(owner)
	}
	return crb
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
