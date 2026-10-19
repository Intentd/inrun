// pkg/resources/resourcequotas/resourcequota.go
package resourcequotas

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Spec is the fully resolved ResourceQuota specification.
type Spec struct {
	Name              string
	Namespace         string
	Hard              map[string]string
	FromResourceQuota string
	FromNamespace     string
	Labels            map[string]string
}

// Create creates a ResourceQuota if it does not already exist.
// Idempotent — skips if it already exists.
// Owner reference set for cascade deletion.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.CoreV1().ResourceQuotas(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("resourcequota.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	hard, err := resolveHard(ctx, kube, spec, owner)
	if err != nil {
		return "", fmt.Errorf("resourcequota.Create: resolving hard limits: %w", err)
	}

	spec.Hard = hard
	rq := Build(owner, spec)

	_, err = kube.CoreV1().ResourceQuotas(namespace).Create(ctx, rq, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("resourcequota.Create: creating %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates a ResourceQuota using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	hard, err := resolveHard(ctx, kube, spec, owner)
	if err != nil {
		return "", fmt.Errorf("resourcequota.Apply: resolving hard limits: %w", err)
	}

	spec.Hard = hard
	rq := Build(owner, spec)
	rq.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ResourceQuota"}

	body, err := json.Marshal(rq)
	if err != nil {
		return "", fmt.Errorf("resourcequota.Apply: marshal: %w", err)
	}

	if _, err = kube.CoreV1().ResourceQuotas(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("resourcequota.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the ResourceQuota if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.CoreV1().ResourceQuotas(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("resourcequota.Delete: deleting %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the ResourceQuota only if it is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().ResourceQuotas(namespace).
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
	if err := kube.CoreV1().ResourceQuotas(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// CopyToNamespaces copies a ResourceQuota to multiple target namespaces.
// Reads the source quota once and creates copies in each namespace.
// Idempotent — skips namespaces where the quota already exists.
func CopyToNamespaces(
	ctx context.Context,
	kube kubernetes.Interface,
	owner types.Owner,
	spec Spec,
	toNamespaces []string,
) error {
	hard, err := resolveHard(ctx, kube, spec, owner)
	if err != nil {
		return fmt.Errorf("resourcequota.CopyToNamespaces: reading source: %w", err)
	}

	for _, ns := range toNamespaces {
		if ns == "" {
			continue
		}

		_, err := kube.CoreV1().ResourceQuotas(ns).Get(ctx, spec.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("resourcequota.CopyToNamespaces: checking %q in %q: %w", spec.Name, ns, err)
		}
		if err == nil {
			continue
		}

		nsSpec := spec
		nsSpec.Namespace = ns
		nsSpec.Hard = hard
		rq := Build(owner, nsSpec)

		_, err = kube.CoreV1().ResourceQuotas(ns).Create(ctx, rq, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("resourcequota.CopyToNamespaces: creating %q in %q: %w", spec.Name, ns, err)
		}

	}

	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// resolveHard returns the hard limits to apply.
// When FromResourceQuota is set, copies limits from the source quota.
// Otherwise uses the declared Hard map.
func resolveHard(
	ctx context.Context,
	kube kubernetes.Interface,
	spec Spec,
	owner types.Owner,
) (map[string]string, error) {
	if spec.FromResourceQuota == "" {
		return spec.Hard, nil
	}

	fromNS := spec.FromNamespace
	if fromNS == "" {
		fromNS = owner.GetNamespace()
	}
	if fromNS == "" {
		fromNS = "default"
	}

	source, err := kube.CoreV1().ResourceQuotas(fromNS).
		Get(ctx, spec.FromResourceQuota, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("reading source resourcequota %q from %q: %w",
			spec.FromResourceQuota, fromNS, err)
	}

	hard := make(map[string]string, len(source.Spec.Hard))
	for k, v := range source.Spec.Hard {
		hard[string(k)] = v.String()
	}
	return hard, nil
}

func buildResourceList(hard map[string]string) corev1.ResourceList {
	rl := make(corev1.ResourceList, len(hard))
	for k, v := range hard {
		q, err := resource.ParseQuantity(v)
		if err != nil {
			continue
		}
		rl[corev1.ResourceName(k)] = q
	}
	return rl
}

// Build returns the ResourceQuota spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.ResourceQuota {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	hard := spec.Hard
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: buildResourceList(hard),
		},
	}
}
