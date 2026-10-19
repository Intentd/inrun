// pkg/resources/secrets/secret.go
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Spec is the fully resolved Secret specification.
type Spec struct {
	// Name — Secret name. Required.
	Name string

	// Namespace — target namespace. Required.
	Namespace string

	// StringData — static key-value entries.
	// Values are plain strings — template expressions are not supported here.
	StringData map[string]string

	// Data
	Data map[string][]byte

	// Type — Kubernetes Secret type.
	// Default: "Opaque"
	Type string

	// FromSecret — name of a source Secret to copy from.
	// When set, copies all keys from the source into the target.
	// Individual Data entries merge on top — override specific keys if needed.
	FromSecret string

	// FromNamespace — namespace where FromSecret lives.
	// Default: same namespace as the CR.
	FromNamespace string

	// Labels — applied to Secret metadata.
	Labels map[string]string

	// Annotations — applied to Secret metadata.
	Annotations map[string]string
}

// Create creates a Secret in the target namespace if it does not already exist.
// When FromSecret is set, copies data from the source Secret automatically.
// Idempotent — skips creation if the Secret already exists.
// Owner reference is set so the Secret is garbage collected when the CR is deleted.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("secret.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.CoreV1().Secrets(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("secret.Create: checking existence of %q in %q: %w", spec.Name, namespace, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	// If copying from another Secret, fetch its data first
	data, stringData, err := resolveData(ctx, kube, spec, owner)
	if err != nil {
		return "", fmt.Errorf("secret.Create: resolving data for %q: %w", spec.Name, err)
	}

	spec.Data = data
	spec.StringData = stringData
	secret := Build(owner, spec)

	_, err = kube.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("secret.Create: creating secret %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates a Secret using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("secret.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	data, stringData, err := resolveData(ctx, kube, spec, owner)
	if err != nil {
		return "", fmt.Errorf("secret.Apply: resolving data for %q: %w", spec.Name, err)
	}

	spec.Data = data
	spec.StringData = stringData
	secret := Build(owner, spec)
	secret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}

	body, err := json.Marshal(secret)
	if err != nil {
		return "", fmt.Errorf("secret.Apply: marshal: %w", err)
	}

	if _, err = kube.CoreV1().Secrets(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("secret.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the Secret if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.CoreV1().Secrets(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("secret.Delete: deleting secret %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// CopyToNamespaces copies a Secret to multiple target namespaces.
// Each copy is owned by the CR — garbage collected when the CR is deleted.
// Idempotent — skips namespaces where the Secret already exists.
// This is the power pattern: one declaration copies to N namespaces automatically.
//
// Example Blueprint declaration:
//
//	onCreate:
//	  secrets:
//	    - name: db-credentials
//	      fromSecret: master-db-creds
//	      fromNamespace: platform
//	      toNamespaces:
//	        - "{{ .metadata.namespace }}"
//	        - monitoring
//	        - staging
func CopyToNamespaces(
	ctx context.Context,
	kube kubernetes.Interface,
	owner types.Owner,
	spec Spec,
	toNamespaces []string,
) error {
	// Fetch source Secret once — reuse data for all target namespaces
	sourceData, sourceStringData, err := resolveData(ctx, kube, spec, owner)
	if err != nil {
		return fmt.Errorf("secret.CopyToNamespaces: reading source %q: %w", spec.FromSecret, err)
	}

	for _, ns := range toNamespaces {
		if ns == "" {
			continue
		}

		nsSpec := spec
		nsSpec.Namespace = ns

		_, err := kube.CoreV1().Secrets(ns).Get(ctx, spec.Name, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("secret.CopyToNamespaces: checking %q in %q: %w", spec.Name, ns, err)
		}
		if err == nil {
			continue
		}

		nsSpec.Data = sourceData
		nsSpec.StringData = sourceStringData
		secret := Build(owner, nsSpec)

		_, err = kube.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("secret.CopyToNamespaces: creating %q in %q: %w", spec.Name, ns, err)
		}

	}

	return nil
}

// DeleteIfOwned deletes the Secret if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.CoreV1().Secrets(namespace).
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
	if err := kube.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// resolveData returns the Secret data to write.
// When FromSecret is set, fetches data from the source Secret.
// Otherwise uses the static data declared in the spec.
func resolveData(
	ctx context.Context,
	kube kubernetes.Interface,
	spec Spec,
	owner types.Owner,
) (map[string][]byte, map[string]string, error) {
	if spec.FromSecret == "" {
		return spec.Data, spec.StringData, nil
	}

	// Determine source namespace
	fromNS := spec.FromNamespace
	if fromNS == "" {
		fromNS = owner.GetNamespace()
	}
	if fromNS == "" {
		fromNS = "default"
	}

	source, err := kube.CoreV1().Secrets(fromNS).Get(ctx, spec.FromSecret, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("reading source secret %q from %q: %w", spec.FromSecret, fromNS, err)
	}

	return source.Data, nil, nil
}

// Build returns the Secret spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *corev1.Secret {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)
	data := spec.Data
	stringData := spec.StringData
	secretType := corev1.SecretTypeOpaque
	switch strings.ToLower(spec.Type) {
	case "kubernetes.io/tls":
		secretType = corev1.SecretTypeTLS
	case "kubernetes.io/dockerconfigjson":
		secretType = corev1.SecretTypeDockerConfigJson
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Type:       secretType,
		Data:       data,
		StringData: stringData,
	}
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if spec.FromSecret == "" && len(spec.Data) == 0 && len(spec.StringData) == 0 {
		return fmt.Errorf("one of fromSecret or data must be declared")
	}
	return nil
}
