// pkg/resources/ingresses/ingress.go
package ingresses

import (
	"context"
	"encoding/json"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/internal/shared"
)

// Create creates an Ingress owned by the CR if it does not already exist.
// Idempotent — if the Ingress exists, does nothing and returns nil.
func Create(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("ingress.Create: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	_, err := kube.NetworkingV1().Ingresses(namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", fmt.Errorf("ingress.Create: checking existence of %q: %w", spec.Name, err)
	}
	if err == nil {
		return types.Unchanged, nil
	}

	ing := Build(owner, spec)

	_, err = kube.NetworkingV1().Ingresses(namespace).Create(ctx, ing, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("ingress.Create: creating ingress %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Created, nil
}

// Apply creates or updates an Ingress using Server-Side Apply.
// Sends only the fields Inrun owns; k8s-injected defaults are invisible.
func Apply(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec, opts types.Options) (types.Result, error) {
	if err := validateSpec(spec); err != nil {
		return "", fmt.Errorf("ingress.Apply: invalid spec: %w", err)
	}

	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	ing := Build(owner, spec)
	ing.TypeMeta = metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"}

	body, err := json.Marshal(ing)
	if err != nil {
		return "", fmt.Errorf("ingress.Apply: marshal: %w", err)
	}

	if _, err = kube.NetworkingV1().Ingresses(namespace).Patch(
		ctx, spec.Name, k8stypes.ApplyPatchType, body,
		opts.PatchOptions(),
	); err != nil {
		return "", fmt.Errorf("ingress.Apply: %w", err)
	}

	return types.Applied, nil
}

// Delete deletes the Ingress if it exists.
func Delete(ctx context.Context, kube kubernetes.Interface, owner types.Owner, spec Spec) (types.Result, error) {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	err := kube.NetworkingV1().Ingresses(namespace).Delete(ctx, spec.Name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return types.NotFound, nil
		}
		return "", fmt.Errorf("ingress.Delete: deleting ingress %q in %q: %w", spec.Name, namespace, err)
	}

	return types.Deleted, nil
}

// DeleteIfOwned deletes the Ingress if it exists and is owned by the CR.
func DeleteIfOwned(ctx context.Context, kube kubernetes.Interface, name, namespace string, owned func(metav1.Object) bool) (types.Result, error) {
	existing, err := kube.NetworkingV1().Ingresses(namespace).
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
	if err := kube.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return "", err
	}
	return types.Deleted, nil
}

// ── Build and its helpers ─────────────────────────────────────────────────

// Build returns the Ingress spec describes, owned by owner, in spec.Namespace or else the owner's namespace.
// It reads nothing from the cluster.
func Build(owner types.Owner, spec Spec) *networkingv1.Ingress {
	namespace := shared.ResolveNamespace(owner, spec.Namespace)

	pathType := networkingv1.PathTypePrefix
	switch spec.PathType {
	case "Exact":
		pathType = networkingv1.PathTypeExact
	case "ImplementationSpecific":
		pathType = networkingv1.PathTypeImplementationSpecific
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            spec.Name,
			Namespace:       namespace,
			Labels:          spec.Labels,
			Annotations:     spec.Annotations,
			OwnerReferences: types.OwnerReferences(owner),
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: spec.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     spec.Path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: spec.ServiceName,
											Port: networkingv1.ServiceBackendPort{
												Number: spec.ServicePort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if spec.IngressClass != "" {
		ing.Spec.IngressClassName = &spec.IngressClass
	}

	if spec.TLS != nil && spec.TLS.Create {
		hosts := spec.TLS.Hosts
		if len(hosts) == 0 && spec.Host != "" {
			hosts = []string{spec.Host}
		}
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      hosts,
				SecretName: spec.TLS.SecretName,
			},
		}
	}

	return ing
}

func validateSpec(spec Spec) error {
	if spec.Name == "" {
		return fmt.Errorf("missing required field: name")
	}
	return nil
}
