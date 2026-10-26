package runners

// Networking resources: Services, Ingresses and NetworkPolicies.

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/gateway/certmanager"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/template/resources"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/kubewright/core/ingresses"
	"github.com/intentd/kubewright/core/networkpolicies"
	"github.com/intentd/kubewright/core/services"
)

// RunServices writes Service template entries; runChildren describes how.
func RunServices(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.ServiceTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, serviceChild)
}

var serviceChild = child[types.ServiceTemplateSource, services.Spec]{
	log:      "service",
	resource: "Service",
	field:    "services",
	entry: func(s types.ServiceTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveService,
	spec:          noPresets(resources.ServiceSpec),
	create:        services.Create,
	apply:         services.Apply,
	deleteIfOwned: services.DeleteIfOwned,
}

// RunIngresses writes Ingress template entries; runChildren describes how.
//
// An entry with tls.create gets a self-signed TLS Secret before its Ingress is written.
func RunIngresses(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.IngressTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, ingressChild)
}

var ingressChild = child[types.IngressTemplateSource, ingresses.Spec]{
	log:      "ingress",
	resource: "Ingress",
	field:    "ingresses",
	entry: func(s types.IngressTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveIngress,
	spec:          noPresets(resources.IngressSpec),
	create:        ingresses.Create,
	apply:         ingresses.Apply,
	deleteIfOwned: ingresses.DeleteIfOwned,
	before: func(ctx context.Context, kube kubeclient.Interface, owner domain.Object, spec ingresses.Spec, ns string) error {
		if spec.TLS == nil || !spec.TLS.Create {
			return nil
		}
		if err := ensureIngressTLSSecret(ctx, kube, owner, spec, ns); err != nil {
			return fmt.Errorf("tls secret: %w", err)
		}
		return nil
	},
}

// ensureIngressTLSSecret creates a kubernetes.io/tls Secret for the Ingress if
// it does not already exist. Idempotent — safe to call on every reconcile.
//
// CN  = first declared host (or Ingress name when Hosts is Empty().
// SANs = all declared hosts.
func ensureIngressTLSSecret(
	ctx context.Context,
	kube kubeclient.Interface,
	owner domain.Object,
	spec ingresses.Spec,
	namespace string,
) error {
	secretName := spec.TLS.SecretName
	if secretName == "" {
		secretName = spec.Name + "-tls"
	}

	// Idempotency check — skip if the Secret already exists.
	_, err := kube.Clientset().CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		logger.FromContext(ctx).Debug().
			Str("secret", secretName).
			Str("namespace", namespace).
			Msg("ingress tls secret already exists — skipping")
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("checking tls secret %q: %w", secretName, err)
	}

	// Determine CN and SANs from the TLS hosts list.
	hosts := spec.TLS.Hosts
	if len(hosts) == 0 && spec.Host != "" {
		hosts = []string{spec.Host}
	}

	cn := secretName
	if len(hosts) > 0 {
		cn = hosts[0]
	}

	bundle, err := certmanager.GenerateTLSBundle(cn, hosts, spec.TLS.ValidFor)
	if err != nil {
		return fmt.Errorf("generating tls bundle for %q: %w", secretName, err)
	}

	return createTLSSecret(ctx, kube, owner, secretName, namespace, "", bundle)
}

// RunNetworkPolicies writes NetworkPolicy template entries; runChildren describes how.
//
// An entry with toNamespaces writes the NetworkPolicy into each of those
// namespaces instead of the owner's.
func RunNetworkPolicies(
	ctx context.Context,
	kube kubeclient.Interface,
	resolver *template.Resolver,
	owner domain.Object,
	srcs []types.NetworkPolicyTemplateSource,
	update bool,
	guard nsGuard,
) error {
	return runChildren(ctx, kube, resolver, owner, srcs, update, guard, networkpolicyChild)
}

var networkpolicyChild = child[types.NetworkPolicyTemplateSource, networkpolicies.Spec]{
	log:      "networkpolicy",
	resource: "NetworkPolicy",
	field:    "networkPolicies",
	entry: func(s types.NetworkPolicyTemplateSource) entryFields {
		return entryFields{Name: s.Name, Namespace: s.Namespace, When: s.When, Or: s.Or, Reconcile: s.Reconcile, ForceConflict: s.ForceConflict}
	},
	resolve:       resources.ResolveNetworkPolicy,
	spec:          resources.NetworkPolicySpec,
	create:        networkpolicies.Create,
	apply:         networkpolicies.Apply,
	deleteIfOwned: networkpolicies.DeleteIfOwned,
	toNamespaces:  func(s types.NetworkPolicyTemplateSource) []string { return s.ToNamespaces },
	inNamespace:   func(spec networkpolicies.Spec, ns string) networkpolicies.Spec { spec.Namespace = ns; return spec },
}
