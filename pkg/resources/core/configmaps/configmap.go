// pkg/resources/configmaps/configmap.go
package configmaps

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
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Spec is the fully resolved ConfigMap specification.
type Spec struct {
	// Name — ConfigMap name. Required.
	Name string

	// Namespace — target namespace. Required.
	Namespace string

	// Data — key-value configuration entries.
	Data map[string]string

	// FromConfigMap — name of a source ConfigMap to copy from.
	// When set, copies all keys from the source into the target.
	// Individual Data entries merge on top — override specific keys if needed.
	FromConfigMap string

	// FromNamespace — namespace where FromConfigMap lives.
	// Default: same namespace as the CR.
	FromNamespace string

	// Labels — applied to ConfigMap metadata.
	Labels map[string]string
}

// Create creates a ConfigMap if it does not already exist.
// Idempotent — skips if ConfigMap exists.
// Owner reference set for cascade deletion.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("configmap.Create: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.CoreV1().ConfigMaps(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("configmap.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	data, err := resolveData(ctx, kube, spec, owner)
	if err != nil {
		return "", fmt.Errorf("configmap.Create: resolving data: %w", err)
	}

	spec.Data = data
	cm := Build(owner, spec)

	_, err = kube.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("configmap.Create: creating %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates a ConfigMap using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("configmap.Apply: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	data, err := resolveData(ctx, kube, spec, owner)
	if err != nil {
		return "", fmt.Errorf("configmap.Apply: resolving data: %w", err)
	}

	spec.Data = data
	cm := Build(owner, spec)
	cm.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}

	body, err := json.Marshal(cm)
	if err != nil {
		return "", fmt.Errorf("configmap.Apply: marshal: %w", err)
	}

	if _, err = kube.CoreV1().ConfigMaps(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("configmap.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the ConfigMap if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.CoreV1().ConfigMaps(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("configmap.Delete: deleting %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// CopyToNamespaces copies a ConfigMap to multiple target namespaces.
// Reads the source once and creates copies in each namespace.
// Idempotent — skips namespaces where the ConfigMap already exists.
//
// Example Blueprint declaration:
//
//	onCreate:
//	  configMaps:
//	    - name: app-config
//	      fromConfigMap: base-app-config
//	      fromNamespace: platform
//	      toNamespaces:
//	        - "{{ .metadata.namespace }}"
//	        - staging
//	        - production
func CopyToNamespaces(
	ctx context.Context,
	kube kubernetes.Interface,
	owner types.Owner,
	spec Spec,
	toNamespaces []string,
) error {
	// Fetch source once — reuse for all target namespaces
	data, err := resolveData(ctx, kube, spec, owner)
	if err != nil {
		return fmt.Errorf("configmap.CopyToNamespaces: reading source: %w", err)
	}

	for _, ns := range toNamespaces {
		if ns == "" {
			continue
		}

		_, err := kube.CoreV1().ConfigMaps(ns).Get(ctx, spec.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("configmap.CopyToNamespaces: checking %q in %q: %w", spec.Name, ns, err)
		}
		if err == nil {
			continue
		}

		nsSpec := spec
		nsSpec.Namespace = ns
		nsSpec.Data = data
		cm := Build(owner, nsSpec)

		_, err = kube.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("configmap.CopyToNamespaces: creating %q in %q: %w", spec.Name, ns, err)
		}

	}

	return nil
}

// DeleteIfOwned ConfigMaps the Job if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().ConfigMaps(namespace).
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
	if err := kube.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// resolveData merges FromConfigMap source data with declared Data.
// Declared Data keys win over source keys — allows targeted overrides.
func resolveData(
	ctx context.Context,
	kube kubernetes.Interface,
	spec Spec,
	owner types.Owner,
) (map[string]string, error) {
	merged := make(map[string]string)

	// Base: copy from source ConfigMap if declared
	if spec.FromConfigMap != "" {
		fromNS := spec.FromNamespace
		if fromNS == "" {
			fromNS = owner.GetNamespace()
		}
		if fromNS == "" {
			fromNS = "default"
		}

		source, err := kube.CoreV1().ConfigMaps(fromNS).Get(ctx, spec.FromConfigMap, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("reading source configmap %q from %q: %w", spec.FromConfigMap, fromNS, err)
		}

		for k, v := range source.Data {
			merged[k] = v
		}

	}

	// Override: declared Data keys win over source keys
	for k, v := range spec.Data {
		merged[k] = v
	}

	return merged, nil
}

// Build returns the ConfigMap spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.ConfigMap {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	data := spec.Data
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Data: data,
	}
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}
