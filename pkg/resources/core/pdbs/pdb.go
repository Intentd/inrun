// pkg/resources/pdbs/pdb.go
package pdbs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Create creates a PDB owned by the CR if it does not already exist.
// Idempotent — if the PDB exists, does nothing and returns nil.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("pdb.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("pdb.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	pdb := Build(owner, spec)

	_, err = kube.PolicyV1().PodDisruptionBudgets(namespace).Create(ctx, pdb, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("pdb.Create: creating pdb %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates a PodDisruptionBudget using Server-Side Apply.
// PDB selectors are immutable — if SSA is rejected, falls back to delete + recreate.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("pdb.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	pdb := Build(owner, spec)
	pdb.TypeMeta = metav1.TypeMeta{APIVersion: "policy/v1", Kind: "PodDisruptionBudget"}

	body, err := json.Marshal(pdb)
	if err != nil {
		return "", fmt.Errorf("pdb.Apply: marshal: %w", err)
	}

	if _, err = kube.PolicyV1().PodDisruptionBudgets(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		if errors.IsInvalid(err) {
			if delErr := kube.PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{}); delErr != nil && !errors.IsNotFound(delErr) {
				return "", fmt.Errorf("pdb.Apply: deleting stale pdb %q: %w", spec.Name, delErr)
			}
			if _, err := Create(ctx, kube, owner, spec); err != nil {
				return "", err
			}
			return types.Replaced, nil
		}
		return "", fmt.Errorf("pdb.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the PDB if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("pdb.Delete: deleting pdb %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the PDB if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.PolicyV1().PodDisruptionBudgets(namespace).
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
	if err := kube.PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the PodDisruptionBudget spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *policyv1.PodDisruptionBudget {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: spec.Selector,
			},
		},
	}

	if spec.MinAvailable != "" {
		v := parseIntOrString(spec.MinAvailable)
		pdb.Spec.MinAvailable = &v
	} else if spec.MaxUnavailable != "" {
		v := parseIntOrString(spec.MaxUnavailable)
		pdb.Spec.MaxUnavailable = &v
	}

	return pdb
}

// parseIntOrString converts a string to intstr.IntOrString.
// Strings ending in "%" are treated as string values; others as integers.
func parseIntOrString(s string) intstr.IntOrString {
	if strings.HasSuffix(s, "%") {
		return intstr.FromString(s)
	}
	if n, err := strconv.Atoi(s); err == nil {
		return intstr.FromInt32(int32(n))
	}
	return intstr.FromString(s)
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("missing required field: name")
	}
	return nil
}
