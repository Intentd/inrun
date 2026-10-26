package resources

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"

	"github.com/intentd/kubewright/core/configmaps"
	"github.com/intentd/kubewright/core/namespaces"
	"github.com/intentd/kubewright/core/secrets"
	"github.com/intentd/kubewright/core/serviceaccounts"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ResolveNamespace resolves all template expressions in a NamespaceTemplateSource.
func ResolveNamespace(r *template.Resolver, src types.NamespaceTemplateSource) (types.NamespaceTemplateSource, error) {
	resolved := types.NamespaceTemplateSource{}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("namespace.name: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("namespace.labels: %w", err)
	}

	for i, a := range src.Finalizers {
		rv, e := r.Resolve(a)
		if e != nil {
			return resolved, fmt.Errorf("namespace.finalizers[%d]: %w", i, e)
		}
		resolved.Finalizers = append(resolved.Finalizers, rv)
	}

	return resolved, nil
}

// ResolveConfigMap resolves all template expressions in a ConfigMapsTemplateSource.
// Returns a new ConfigMapTemplateSource with all expressions evaluated — safe to pass
// directly to configmaps.Resolve().
func ResolveConfigMap(r *template.Resolver, src types.ConfigMapTemplateSource) (types.ConfigMapTemplateSource, error) {
	resolved := types.ConfigMapTemplateSource{}
	var err error

	// Resolve the template expressions
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("configmap.name: %w", err)
	}
	if resolved.Namespace, err = r.Resolve(src.Namespace); err != nil {
		return resolved, fmt.Errorf("configmap.namespace: %w", err)
	}
	if resolved.FromNamespace, err = r.Resolve(src.FromNamespace); err != nil {
		return resolved, fmt.Errorf("configmap.fromNamespace: %w", err)
	}
	if resolved.FromConfigMap, err = r.Resolve(src.FromConfigMap); err != nil {
		return resolved, fmt.Errorf("configmap.fromConfigMap: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("configmap.labels: %w", err)
	}

	if len(src.Data) > 0 {
		resolved.Data = make(map[string]string, len(src.Data))
		for k, v := range src.Data {
			rv, e := r.Resolve(v)
			if e != nil {
				return resolved, fmt.Errorf("configmap.data[%q]: %w", k, e)
			}
			resolved.Data[k] = rv
		}
	}

	// toNamespaces needs special handling.
	// Each element is either:
	//   a) A literal namespace name → resolve as string
	//   b) A template expression that resolves to a string → resolve as string
	//   c) A template expression that resolves to a []interface{} (list field) →
	//      extract each element individually
	//
	// Case (c) is what happens when toNamespaces: ["{{ .spec.targetNamespaces }}"]
	// where .spec.targetNamespaces is a YAML list in the CR.

	for i, v := range src.ToNamespaces {
		if !types.IsTemplate(v) {
			// Static string — no resolution needed
			if v != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, v)
			}
			continue
		}

		// Template expression — check if it resolves to a list field
		raw := r.ResolveRaw(v)
		switch typed := raw.(type) {
		case []interface{}:
			// List field — extract each string element
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
			// Fall back to string resolution
			rv, e := r.Resolve(v)
			if e != nil {
				return resolved, fmt.Errorf("secret.toNamespaces[%d]: %w", i, e)
			}
			if rv != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, rv)
			}
		}
	}

	return resolved, nil
}

// ResolveSecret resolves all template expressions in a SecretTemplateSource.
// Returns a new SecretTemplateSource with all expressions evaluated — safe to pass
// directly to secrets.Resolve().
func ResolveSecret(r *template.Resolver, src types.SecretTemplateSource) (types.SecretTemplateSource, error) {
	resolved := types.SecretTemplateSource{}

	var err error

	// Resolve the template expressions
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("secret.name: %w", err)
	}
	if resolved.FromSecret, err = r.Resolve(src.FromSecret); err != nil {
		return resolved, fmt.Errorf("secret.fromSecret: %w", err)
	}
	if resolved.FromNamespace, err = r.Resolve(src.FromNamespace); err != nil {
		return resolved, fmt.Errorf("secret.fromNamespace: %w", err)
	}
	if resolved.Type, err = r.Resolve(src.Type); err != nil {
		return resolved, fmt.Errorf("secret.type: %w", err)
	}
	if resolved.Namespace, err = r.Resolve(src.Namespace); err != nil {
		return resolved, fmt.Errorf("secret.namespace: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("secret.data: %w", err)
	}

	if len(src.Data) > 0 {
		resolved.Data = make(map[string]string, len(src.Data))
		for k, v := range src.Data {
			rv, e := r.Resolve(v)
			if e != nil {
				return resolved, fmt.Errorf("secret.data[%q]: %w", k, e)
			}
			resolved.Data[k] = rv
		}
	}

	// toNamespaces needs special handling.
	// Each element is either:
	//   a) A literal namespace name → resolve as string
	//   b) A template expression that resolves to a string → resolve as string
	//   c) A template expression that resolves to a []interface{} (list field) →
	//      extract each element individually
	//
	// Case (c) is what happens when toNamespaces: ["{{ .spec.targetNamespaces }}"]
	// where .spec.targetNamespaces is a YAML list in the CR.

	for i, v := range src.ToNamespaces {
		if !types.IsTemplate(v) {
			// Static string — no resolution needed
			if v != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, v)
			}
			continue
		}

		// Template expression — check if it resolves to a list field
		raw := r.ResolveRaw(v)
		switch typed := raw.(type) {
		case []interface{}:
			// List field — extract each string element
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
			// Fall back to string resolution
			rv, e := r.Resolve(v)
			if e != nil {
				return resolved, fmt.Errorf("secret.toNamespaces[%d]: %w", i, e)
			}
			if rv != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, rv)
			}
		}
	}
	return resolved, nil
}

// ResolveServiceAccount resolves all template expressions in a ServiceAccountTemplateSource.
func ResolveServiceAccount(r *template.Resolver, src types.ServiceAccountTemplateSource) (types.ServiceAccountTemplateSource, error) {
	resolved := types.ServiceAccountTemplateSource{}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("serviceaccount.name: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("serviceaccount.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("serviceaccount.labels: %w", err)
	}

	return resolved, nil
}

// ConfigMapSpec builds a configmaps.Spec from a ConfigMapTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func ConfigMapSpec(src types.ConfigMapTemplateSource, owner domain.Object) configmaps.Spec {
	ownerName := owner.GetName()
	spec := configmaps.Spec{
		Name:          src.Name,
		Namespace:     src.Namespace,
		Data:          src.Data,
		FromConfigMap: src.FromConfigMap,
		FromNamespace: src.FromNamespace,
		Labels:        make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-config"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// NamespaceSpec builds a namespaces.Spec from a NamespaceTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func NamespaceSpec(src types.NamespaceTemplateSource, owner domain.Object) namespaces.Spec {
	ownerName := owner.GetName()
	spec := namespaces.Spec{
		Name:       src.Name,
		Labels:     make(map[string]string),
		Finalizers: src.Finalizers,
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-ns"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// SecretSpec builds a secrets.Spec from a SecretTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func SecretSpec(src types.SecretTemplateSource, owner domain.Object) secrets.Spec {
	ownerName := owner.GetName()
	spec := secrets.Spec{
		Name:          src.Name,
		Namespace:     src.Namespace,
		FromSecret:    src.FromSecret,
		FromNamespace: src.FromNamespace,
		Type:          src.Type,
		StringData:    src.Data, // declared as strings in YAML
		Labels:        make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-secret"
	}
	if spec.Type == "" {
		spec.Type = "Opaque"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	// System labels

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}

// ServiceAccountSpec builds a serviceaccounts.Spec from a ServiceAccountTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func ServiceAccountSpec(src types.ServiceAccountTemplateSource, owner domain.Object) serviceaccounts.Spec {
	ownerName := owner.GetName()
	spec := serviceaccounts.Spec{
		Name:      src.Name,
		Namespace: src.Namespace,
		Labels:    make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-sa"
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner)
	return spec
}
