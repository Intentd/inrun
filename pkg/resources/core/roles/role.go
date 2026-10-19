// pkg/resources/roles/role.go
package roles

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

// Spec is the fully resolved Role specification.
type Spec struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Rules     []rbacv1.PolicyRule

	// Sleep injects an artificial delay into the reconcile of this resource.
	// Useful for autoscale testing, latency simulation, and chaos engineering.
	// Accepts extended duration units (s, m, h, d, w, mo, y).
	Sleep string

	// ForceConflict, when true, sets Force: true when applying this resource,
	// taking ownership of conflicting fields instead of returning a conflict error.
	// Overrides the CRD-level ForceConflict setting.
	ForceConflict *bool
}

// Create creates a Role if it does not already exist.
// Idempotent — skips if the Role already exists.
// Owner reference ensures cleanup when the CR is deleted.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	if err := validateSpec(spec); err != nil {
		return fmt.Errorf("role.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	_, err := kube.RbacV1().Roles(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("role.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		logger.Debug().
			Str("role", spec.Name).
			Str("namespace", namespace).
			Msg("role already exists — skipping create")
		return nil
	}

	role := Build(owner, spec)

	_, err = kube.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("role.Create: creating role %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("role", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("role created")

	return nil
}

// Apply creates or updates a Role using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	role := Build(owner, spec)
	role.TypeMeta = metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "Role"}

	body, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("role.Apply: marshal: %w", err)
	}

	if _, err = kube.RbacV1().Roles(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return fmt.Errorf("role.Apply: %w", err)
	}

	logger.Debug().
		Str("role", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("role applied")

	return nil
}

// Delete deletes the Role if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) error {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	if err := shared.SleepIfNeeded(spec.Sleep); err != nil {
		return err
	}

	err := kube.RbacV1().Roles(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Debug().
				Str("role", spec.Name).
				Str("namespace", namespace).
				Msg("role already deleted — skipping")
			return nil
		}
		return fmt.Errorf("role.Delete: deleting role %q in %q: %w", spec.Name, namespace, err)
	}

	logger.Info().
		Str("role", spec.Name).
		Str("namespace", namespace).
		Str("owner", owner.GetName()).
		Msg("role deleted")

	return nil
}

// DeleteIfOwned deletes the Role only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface,
	owner types.Owner, name, namespace string) error {

	existing, err := kube.RbacV1().Roles(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if existing.Labels[labels.InrunOwner] != labels.EffectiveOwnerKey(owner.GetName(), owner.GetAnnotations()) {
		return nil
	}
	return kube.RbacV1().Roles(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the Role spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *rbacv1.Role {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: shared.ResolveOwnerReferences(owner),
		},
		Rules: spec.Rules,
	}
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
