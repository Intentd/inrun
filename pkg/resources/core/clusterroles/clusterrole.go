// pkg/resources/clusterroles/clusterrole.go
package clusterroles

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

// Spec is the fully resolved ClusterRole specification.
type Spec struct {
	Name   string
	Labels map[string]string
	Rules  []rbacv1.PolicyRule
	Sleep  string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a ClusterRole if it does not already exist.
// Idempotent — skips if the ClusterRole already exists.
// ClusterRoles are cluster-scoped; ownership is tracked via the inrun.dev/owner label.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("clusterrole.Create: invalid spec: %w", err)
	}
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.RbacV1().ClusterRoles().Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("clusterrole.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("clusterrole", spec.Name).
			Msg("clusterrole already exists — skipping create")
		return nil
	}

	cr := Build(owner, spec)

	_, err = kube.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("clusterrole.Create: creating %q: %w", spec.Name, err)
	}

	logger.Info().
		Str("clusterrole", spec.Name).
		Str("owner", owner.GetName()).
		Msg("clusterrole created")

	return nil
}

// Apply creates or updates a ClusterRole using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) error {
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	cr := Build(owner, spec)
	cr.TypeMeta = metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"}

	body, err := json.Marshal(cr)
	if err != nil {
		return fmt.Errorf("clusterrole.Apply: marshal: %w", err)
	}

	if _, err = kube.RbacV1().ClusterRoles().Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return fmt.Errorf("clusterrole.Apply: %w", err)
	}

	logger.Debug().
		Str("clusterrole", spec.Name).
		Str("owner", owner.GetName()).
		Msg("clusterrole applied")

	return nil
}

// Delete deletes the ClusterRole if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.RbacV1().ClusterRoles().Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("clusterrole.Delete: deleting %q: %w", spec.Name, err)
	}

	logger.Info().
		Str("clusterrole", spec.Name).
		Str("owner", owner.GetName()).
		Msg("clusterrole deleted")

	return nil
}

// DeleteIfOwned deletes the ClusterRole only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface,
	owner types.Owner, name string) error {

	existing, err := kube.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the ClusterRole spec describes, owned by owner.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *rbacv1.ClusterRole {
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   spec.Name,
			Labels: spec.Labels,
		},
		Rules: spec.Rules,
	}
	if owner.GetNamespace() == "" {
		cr.OwnerReferences = shared.ResolveOwnerReferences(owner)
	}
	return cr
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
