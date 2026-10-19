// pkg/resources/namespace/namespace.go
package namespaces

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
)

// Spec is the fully resolved Namespace specification.
type Spec struct {
	// Name — name. Required.
	Name string

	// Labels — applied to Namespace metadata.
	Labels map[string]string

	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage.
	// optional
	Finalizers []string
}

// Create creates a Namespace if it does not already exist.
// Idempotent — skips if the Namespace exists.
// Owner reference set so Namespace is garbage collected when CR is deleted.
//
// Namespaces have no meaningful spec fields that can drift after creation.
// There is no Update function — Create is called from both onCreate and
// onReconcile paths. If it exists, it stays. If it was deleted, it is recreated.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("namespace.Create: invalid spec: %w", err)
	}

	_, err := kube.CoreV1().Namespaces().Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("namespace.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	sa := Build(owner, spec)

	_, err = kube.CoreV1().Namespaces().Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("namespace.Create: creating namespace %q: %w", spec.Name, err)
	}

	return types.Created, nil
}

// Apply creates or updates a Namespace using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("namespace.Apply: invalid spec: %w", err)
	}

	ns := Build(owner, spec)
	ns.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"}

	body, err := json.Marshal(ns)
	if err != nil {
		return "", fmt.Errorf("namespace.Apply: marshal: %w", err)
	}

	if _, err = kube.CoreV1().Namespaces().Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("namespace.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the Namespace if it exists.
// For most cases owner references handle cleanup automatically —
// only use this when explicit cleanup control is needed.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {

	err := kube.CoreV1().Namespaces().Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("namespace.Delete: deleting namespace %q: %w", spec.Name, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the Namespace if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().Namespaces().
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
	if err := kube.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the Namespace spec describes, owned by owner.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   spec.Name,
			Labels: spec.Labels,
		},
	}
	// Cluster-scoped owners can hold owner references on cluster-scoped resources.
	// Namespace-scoped owners cannot — GC would treat the namespace as orphaned and
	// delete it immediately. Fall back to label-based tracking in that case.
	if owner.GetNamespace() == "" {
		ns.OwnerReferences = types.OwnerReferences(owner)
	}
	return ns
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
