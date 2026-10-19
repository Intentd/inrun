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

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// Spec is the fully resolved ClusterRole specification.
type Spec struct {
	Name   string
	Labels map[string]string
	Rules  []rbacv1.PolicyRule
}

// Create creates a ClusterRole if it does not already exist.
// Idempotent — skips if the ClusterRole already exists.
// ClusterRoles are cluster-scoped; ownership is tracked via the inrun.dev/owner label.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("clusterrole.Create: invalid spec: %w", err)
	}

	_, err := kube.RbacV1().ClusterRoles().Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("clusterrole.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	cr := Build(owner, spec)

	_, err = kube.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("clusterrole.Create: creating %q: %w", spec.Name, err)
	}

	return types.Created, nil
}

// Apply creates or updates a ClusterRole using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {

	cr := Build(owner, spec)
	cr.TypeMeta = metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"}

	body, err := json.Marshal(cr)
	if err != nil {
		return "", fmt.Errorf("clusterrole.Apply: marshal: %w", err)
	}

	if _, err = kube.RbacV1().ClusterRoles().Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("clusterrole.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the ClusterRole if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {

	err := kube.RbacV1().ClusterRoles().Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("clusterrole.Delete: deleting %q: %w", spec.Name, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the ClusterRole only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", err
	}
	if !owned(existing) {
		return types.NotOwned, nil
	}
	if err := kube.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the ClusterRole spec describes, owned by owner.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *rbacv1.ClusterRole {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   spec.Name,
			Labels: spec.Labels,
		},
		Rules: spec.Rules,
	}
	if owner.GetNamespace() == "" {
		cr.OwnerReferences = types.OwnerReferences(owner)
	}
	return cr
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
