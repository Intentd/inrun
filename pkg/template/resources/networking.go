package resources

import (
	"fmt"
	"strconv"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/kuberesources/core/ingresses"
	"github.com/intentd/kuberesources/core/networkpolicies"
	"github.com/intentd/kuberesources/core/services"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ResolveService resolves all template expressions in a ServiceTemplateSource.
func ResolveService(r *template.Resolver, src types.ServiceTemplateSource) (types.ServiceTemplateSource, error) {
	resolved := types.ServiceTemplateSource{}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("service.name: %w", err)
	}
	if resolved.Type, err = r.Resolve(src.Type); err != nil {
		return resolved, fmt.Errorf("service.type: %w", err)
	}
	if resolved.Protocol, err = r.Resolve(src.Protocol); err != nil {
		return resolved, fmt.Errorf("service.protocol: %w", err)
	}
	if resolved.Port, err = r.Resolve(src.Port); err != nil {
		return resolved, fmt.Errorf("service.port: %w", err)
	}
	if resolved.TargetPort, err = r.Resolve(src.TargetPort); err != nil {
		return resolved, fmt.Errorf("service.targetPort: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("service.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("service.labels: %w", err)
	}

	if resolved.Selector, err = r.ResolveMap(src.Selector); err != nil {
		return resolved, fmt.Errorf("service.selector: %w", err)
	}

	return resolved, nil
}

// ResolveIngress resolves all template expressions in an IngressTemplateSource.
func ResolveIngress(r *template.Resolver, src types.IngressTemplateSource) (types.IngressTemplateSource, error) {
	resolved := types.IngressTemplateSource{}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("ingress.name: %w", err)
	}
	if resolved.Host, err = r.Resolve(src.Host); err != nil {
		return resolved, fmt.Errorf("ingress.host: %w", err)
	}
	if resolved.ServiceName, err = r.Resolve(src.ServiceName); err != nil {
		return resolved, fmt.Errorf("ingress.serviceName: %w", err)
	}
	if resolved.ServicePort, err = r.Resolve(src.ServicePort); err != nil {
		return resolved, fmt.Errorf("ingress.servicePort: %w", err)
	}
	if resolved.Path, err = r.Resolve(src.Path); err != nil {
		return resolved, fmt.Errorf("ingress.path: %w", err)
	}
	if resolved.PathType, err = r.Resolve(src.PathType); err != nil {
		return resolved, fmt.Errorf("ingress.pathType: %w", err)
	}
	if resolved.IngressClass, err = r.Resolve(src.IngressClass); err != nil {
		return resolved, fmt.Errorf("ingress.ingressClass: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("ingress.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("ingress.labels: %w", err)
	}
	if resolved.Annotations, err = r.ResolveMap(src.Annotations); err != nil {
		return resolved, fmt.Errorf("ingress.annotations: %w", err)
	}

	if src.TLS != nil {
		resolvedTLS := &types.IngressTLSSpec{
			Create:   src.TLS.Create,
			ValidFor: src.TLS.ValidFor,
		}
		if resolvedTLS.SecretName, err = r.Resolve(src.TLS.SecretName); err != nil {
			return resolved, fmt.Errorf("ingress.tls.secretName: %w", err)
		}
		for i, h := range src.TLS.Hosts {
			rv, e := r.Resolve(h)
			if e != nil {
				return resolved, fmt.Errorf("ingress.tls.hosts[%d]: %w", i, e)
			}
			resolvedTLS.Hosts = append(resolvedTLS.Hosts, rv)
		}
		resolved.TLS = resolvedTLS
	}

	return resolved, nil
}

// ResolveNetworkPolicy resolves all template expressions in a NetworkPolicyTemplateSource.
func ResolveNetworkPolicy(r *template.Resolver, src types.NetworkPolicyTemplateSource) (types.NetworkPolicyTemplateSource, error) {
	resolved := types.NetworkPolicyTemplateSource{
		PolicyTypes: src.PolicyTypes,
		Ingress:     src.Ingress,
		Egress:      src.Egress,
		PodSelector: src.PodSelector,
	}
	var err error

	if resolved.Preset, err = r.Resolve(src.Preset); err != nil {
		return resolved, fmt.Errorf("networkpolicy.preset: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("networkpolicy.name: %w", err)
	}
	if resolved.Namespace, err = r.Resolve(src.Namespace); err != nil {
		return resolved, fmt.Errorf("networkpolicy.namespace: %w", err)
	}
	if resolved.FromNetworkPolicy, err = r.Resolve(src.FromNetworkPolicy); err != nil {
		return resolved, fmt.Errorf("networkpolicy.fromNetworkPolicy: %w", err)
	}
	if resolved.FromNamespace, err = r.Resolve(src.FromNamespace); err != nil {
		return resolved, fmt.Errorf("networkpolicy.fromNamespace: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("networkpolicy.labels: %w", err)
	}

	for i, v := range src.ToNamespaces {
		if !types.IsTemplate(v) {
			if v != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, v)
			}
			continue
		}
		raw := r.ResolveRaw(v)
		switch typed := raw.(type) {
		case []interface{}:
			for _, item := range typed {
				if s, ok := item.(string); ok && s != "" {
					resolved.ToNamespaces = append(resolved.ToNamespaces, s)
				}
			}
		case string:
			if typed != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, typed)
			}
		default:
			rv, e := r.Resolve(v)
			if e != nil {
				return resolved, fmt.Errorf("networkpolicy.toNamespaces[%d]: %w", i, e)
			}
			if rv != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, rv)
			}
		}
	}

	return resolved, nil
}

// IngressSpec builds a ingresses.Spec from an IngressTemplateSource.
// All template expressions must be evaluated before calling here.
func IngressSpec(src types.IngressTemplateSource, owner domain.Object) ingresses.Spec {
	ownerName := owner.GetName()
	spec := ingresses.Spec{
		Name:         src.Name,
		Namespace:    src.Namespace,
		Host:         src.Host,
		ServiceName:  src.ServiceName,
		Path:         src.Path,
		PathType:     src.PathType,
		IngressClass: src.IngressClass,
		Labels:       make(map[string]string),
		Annotations:  make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-ingress"
	}
	if spec.Path == "" {
		spec.Path = "/"
	}
	if spec.PathType == "" {
		spec.PathType = "Prefix"
	}

	if src.ServicePort != "" {
		if p, err := strconv.ParseInt(src.ServicePort, 10, 32); err == nil {
			spec.ServicePort = int32(p)
		}
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}
	for k, v := range src.Annotations {
		spec.Annotations[k] = v
	}

	// System labels

	if src.TLS != nil && src.TLS.Create {
		secretName := src.TLS.SecretName
		if secretName == "" {
			secretName = ownerName + "-tls"
		}
		spec.TLS = &ingresses.TLS{
			Create:     true,
			SecretName: secretName,
			Hosts:      src.TLS.Hosts,
			ValidFor:   src.TLS.ValidFor,
		}
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// NetworkPolicySpec builds a networkpolicies.Spec from a NetworkPolicyTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func NetworkPolicySpec(src types.NetworkPolicyTemplateSource, owner domain.Object, reg *types.PresetRegistry) networkpolicies.Spec {
	ingress := src.Ingress
	egress := src.Egress
	policyTypes := src.PolicyTypes

	if src.Preset != "" {
		if p, ok := reg.All().NetworkPolicy(src.Preset); !ok {
			logger.Warn().Str("preset", src.Preset).Msg("unknown networkpolicy preset — skipping")
		} else {
			ingress = p.Ingress
			egress = p.Egress
			policyTypes = p.PolicyTypes
		}
	}

	spec := networkpolicies.Spec{
		Name:              src.Name,
		Namespace:         src.Namespace,
		PodSelector:       src.PodSelector,
		Ingress:           ingress,
		Egress:            egress,
		PolicyTypes:       policyTypes,
		FromNetworkPolicy: src.FromNetworkPolicy,
		FromNamespace:     src.FromNamespace,
		Labels:            make(map[string]string),
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// ServiceSpec builds a services.Spec from a ServiceTemplateSource.
// All fields already resolved by template.Resolver before calling here.
// This function assembles the spec and applies defaults.
func ServiceSpec(src types.ServiceTemplateSource, owner domain.Object) services.Spec {
	ownerName := owner.GetName()
	spec := services.Spec{
		Name:     src.Name,
		Labels:   make(map[string]string),
		Selector: make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-svc"
	}

	spec.Namespace = src.Namespace

	spec.Type = src.Type
	if spec.Type == "" {
		spec.Type = "ClusterIP"
	}

	if src.Port != "" {
		if p, err := strconv.ParseInt(src.Port, 10, 32); err == nil {
			spec.Port = int32(p)
		}
	}

	if src.TargetPort != "" {
		if p, err := strconv.ParseInt(src.TargetPort, 10, 32); err == nil {
			spec.TargetPort = int32(p)
		}
	}

	// If TargetPort not set, default to Port
	if spec.TargetPort == 0 {
		spec.TargetPort = spec.Port
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	for k, v := range src.Selector {
		spec.Selector[k] = v
	}
	spec.Selector[labels.InrunOwner] = owner.GetName()

	// System labels

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}
