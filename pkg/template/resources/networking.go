package resources

import (
	"fmt"
	"strconv"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/kubewright/core/ingresses"
	"github.com/intentd/kubewright/core/networkpolicies"
	"github.com/intentd/kubewright/core/services"
	"github.com/intentd/kubewright/presets"

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

	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
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

	if resolved.Namespace, err = r.ResolveNamespace(src.Namespace); err != nil {
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
	}
	var err error

	for i, rule := range src.Ingress {
		from, err := resolvePeers(r, rule.From)
		if err != nil {
			return resolved, fmt.Errorf("networkpolicy.ingress[%d].from%w", i, err)
		}
		ports, err := resolvePorts(r, rule.Ports)
		if err != nil {
			return resolved, fmt.Errorf("networkpolicy.ingress[%d].ports%w", i, err)
		}
		resolved.Ingress = append(resolved.Ingress, types.NetworkPolicyIngressRule{From: from, Ports: ports})
	}
	for i, rule := range src.Egress {
		to, err := resolvePeers(r, rule.To)
		if err != nil {
			return resolved, fmt.Errorf("networkpolicy.egress[%d].to%w", i, err)
		}
		ports, err := resolvePorts(r, rule.Ports)
		if err != nil {
			return resolved, fmt.Errorf("networkpolicy.egress[%d].ports%w", i, err)
		}
		resolved.Egress = append(resolved.Egress, types.NetworkPolicyEgressRule{To: to, Ports: ports})
	}

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
	if resolved.PodSelector, err = resolveSelector(r, src.PodSelector); err != nil {
		return resolved, fmt.Errorf("networkpolicy.podSelector: %w", err)
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

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// resolvePeers resolves the selectors and ipBlock of a rule's from or to
// peers. An error names the peer, for the caller to prefix.
func resolvePeers(r *template.Resolver, peers []types.NetworkPolicyPeer) ([]types.NetworkPolicyPeer, error) {
	var out []types.NetworkPolicyPeer
	for i, p := range peers {
		var (
			peer types.NetworkPolicyPeer
			err  error
		)
		if peer.PodSelector, err = resolveSelector(r, p.PodSelector); err != nil {
			return nil, fmt.Errorf("[%d].podSelector: %w", i, err)
		}
		if peer.NamespaceSelector, err = resolveSelector(r, p.NamespaceSelector); err != nil {
			return nil, fmt.Errorf("[%d].namespaceSelector: %w", i, err)
		}
		if p.IPBlock != nil {
			block := types.NetworkPolicyIPBlock{}
			if block.CIDR, err = r.Resolve(p.IPBlock.CIDR); err != nil {
				return nil, fmt.Errorf("[%d].ipBlock.cidr: %w", i, err)
			}
			for j, e := range p.IPBlock.Except {
				v, err := r.Resolve(e)
				if err != nil {
					return nil, fmt.Errorf("[%d].ipBlock.except[%d]: %w", i, j, err)
				}
				block.Except = append(block.Except, v)
			}
			peer.IPBlock = &block
		}
		out = append(out, peer)
	}
	return out, nil
}

// resolveSelector resolves a label selector, keeping an absent one absent: a
// peer without a namespaceSelector means its own namespace, while an empty one
// means every namespace.
func resolveSelector(r *template.Resolver, m map[string]string) (map[string]string, error) {
	if m == nil {
		return nil, nil
	}
	return r.ResolveMap(m)
}

// resolvePorts resolves a rule's ports. An error names the port, for the
// caller to prefix.
func resolvePorts(r *template.Resolver, ports []types.NetworkPolicyPort) ([]types.NetworkPolicyPort, error) {
	var out []types.NetworkPolicyPort
	for i, p := range ports {
		var (
			port types.NetworkPolicyPort
			err  error
		)
		if port.Protocol, err = r.Resolve(p.Protocol); err != nil {
			return nil, fmt.Errorf("[%d].protocol: %w", i, err)
		}
		if port.Port, err = r.Resolve(p.Port); err != nil {
			return nil, fmt.Errorf("[%d].port: %w", i, err)
		}
		out = append(out, port)
	}
	return out, nil
}

// NetworkPolicySpec builds a networkpolicies.Spec from a NetworkPolicyTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func NetworkPolicySpec(src types.NetworkPolicyTemplateSource, owner domain.Object, reg *types.PresetRegistry) networkpolicies.Spec {
	rules := presetNetworkPolicy(src.Preset, presets.NetworkPolicyRules{
		Ingress:     src.Ingress,
		Egress:      src.Egress,
		PolicyTypes: src.PolicyTypes,
		PodSelector: src.PodSelector,
	}, reg.All())

	spec := networkpolicies.Spec{
		Name:              src.Name,
		Namespace:         src.Namespace,
		PodSelector:       rules.PodSelector,
		Ingress:           rules.Ingress,
		Egress:            rules.Egress,
		PolicyTypes:       rules.PolicyTypes,
		FromNetworkPolicy: src.FromNetworkPolicy,
		FromNamespace:     src.FromNamespace,
		Labels:            make(map[string]string),
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
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

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}
