package resources

import (
	"fmt"
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/labels"
	"strconv"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/resources/core/hpas"
	"github.com/intentd/inrun/pkg/resources/core/limitranges"
	"github.com/intentd/inrun/pkg/resources/core/pdbs"
	"github.com/intentd/inrun/pkg/resources/core/resourcequotas"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// ResolveHPA resolves all template expressions in an HPATemplateSource.
func ResolveHPA(r *template.Resolver, src types.HPATemplateSource) (types.HPATemplateSource, error) {
	resolved := types.HPATemplateSource{}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("hpa.name: %w", err)
	}
	if resolved.ScaleTargetRef.APIVersion, err = r.Resolve(src.ScaleTargetRef.APIVersion); err != nil {
		return resolved, fmt.Errorf("hpa.scaledRef.apiVersion: %w", err)
	}
	if resolved.ScaleTargetRef.Kind, err = r.Resolve(src.ScaleTargetRef.Kind); err != nil {
		return resolved, fmt.Errorf("hpa.scaledRef.kind: %w", err)
	}
	if resolved.ScaleTargetRef.Name, err = r.Resolve(src.ScaleTargetRef.Name); err != nil {
		return resolved, fmt.Errorf("hpa.scaledRef.name: %w", err)
	}
	if resolved.MinReplicas, err = r.Resolve(src.MinReplicas); err != nil {
		return resolved, fmt.Errorf("hpa.minReplicas: %w", err)
	}
	if resolved.MaxReplicas, err = r.Resolve(src.MaxReplicas); err != nil {
		return resolved, fmt.Errorf("hpa.maxReplicas: %w", err)
	}
	if resolved.TargetCPUUtilizationPercentage, err = r.Resolve(src.TargetCPUUtilizationPercentage); err != nil {
		return resolved, fmt.Errorf("hpa.targetCPUUtilizationPercentage: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("hpa.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("hpa.labels: %w", err)
	}

	if src.Behavior != nil {
		b := *src.Behavior
		resolved.Behavior = &b
	}

	return resolved, nil
}

// ResolvePDB resolves all template expressions in a PDBTemplateSource.
func ResolvePDB(r *template.Resolver, src types.PDBTemplateSource) (types.PDBTemplateSource, error) {
	resolved := types.PDBTemplateSource{}

	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("pdb.name: %w", err)
	}
	if resolved.MinAvailable, err = r.Resolve(src.MinAvailable); err != nil {
		return resolved, fmt.Errorf("pdb.minAvailable: %w", err)
	}
	if resolved.MaxUnavailable, err = r.Resolve(src.MaxUnavailable); err != nil {
		return resolved, fmt.Errorf("pdb.maxUnavailable: %w", err)
	}

	ns := src.Namespace
	if ns == "" {
		ns = "{{ .metadata.namespace }}"
	}
	if resolved.Namespace, err = r.Resolve(ns); err != nil {
		return resolved, fmt.Errorf("pdb.namespace: %w", err)
	}

	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("pdb.labels: %w", err)
	}

	if resolved.Selector, err = r.ResolveMap(src.Selector); err != nil {
		return resolved, fmt.Errorf("pdb.selector: %w", err)
	}

	if src.Behavior != nil {
		b := *src.Behavior
		resolved.Behavior = &b
	}

	return resolved, nil
}

// ResolveResourceQuota resolves all template expressions in a ResourceQuotaTemplateSource.
func ResolveResourceQuota(r *template.Resolver, src types.ResourceQuotaTemplateSource) (types.ResourceQuotaTemplateSource, error) {
	resolved := types.ResourceQuotaTemplateSource{}
	var err error

	if resolved.Preset, err = r.Resolve(src.Preset); err != nil {
		return resolved, fmt.Errorf("resourcequota.preset: %w", err)
	}
	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("resourcequota.name: %w", err)
	}
	if resolved.Namespace, err = r.Resolve(src.Namespace); err != nil {
		return resolved, fmt.Errorf("resourcequota.namespace: %w", err)
	}
	if resolved.FromResourceQuota, err = r.Resolve(src.FromResourceQuota); err != nil {
		return resolved, fmt.Errorf("resourcequota.fromResourceQuota: %w", err)
	}
	if resolved.FromNamespace, err = r.Resolve(src.FromNamespace); err != nil {
		return resolved, fmt.Errorf("resourcequota.fromNamespace: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("resourcequota.labels: %w", err)
	}

	if len(src.Hard) > 0 {
		resolved.Hard = make(map[string]string, len(src.Hard))
		for k, v := range src.Hard {
			rv, e := r.Resolve(v)
			if e != nil {
				return resolved, fmt.Errorf("resourcequota.hard[%q]: %w", k, e)
			}
			resolved.Hard[k] = rv
		}
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
				return resolved, fmt.Errorf("resourcequota.toNamespaces[%d]: %w", i, e)
			}
			if rv != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, rv)
			}
		}
	}

	return resolved, nil
}

// ResolveLimitRange resolves all template expressions in a LimitRangeTemplateSource.
func ResolveLimitRange(r *template.Resolver, src types.LimitRangeTemplateSource) (types.LimitRangeTemplateSource, error) {
	resolved := types.LimitRangeTemplateSource{}
	var err error

	if resolved.Name, err = r.Resolve(src.Name); err != nil {
		return resolved, fmt.Errorf("limitrange.name: %w", err)
	}
	if resolved.Namespace, err = r.Resolve(src.Namespace); err != nil {
		return resolved, fmt.Errorf("limitrange.namespace: %w", err)
	}
	if resolved.FromLimitRange, err = r.Resolve(src.FromLimitRange); err != nil {
		return resolved, fmt.Errorf("limitrange.fromLimitRange: %w", err)
	}
	if resolved.FromNamespace, err = r.Resolve(src.FromNamespace); err != nil {
		return resolved, fmt.Errorf("limitrange.fromNamespace: %w", err)
	}
	if resolved.Labels, err = r.ResolveMap(src.Labels); err != nil {
		return resolved, fmt.Errorf("limitrange.labels: %w", err)
	}

	for i, item := range src.Limits {
		ri := item
		ri.Max, err = resolveStringMap(r, item.Max)
		if err != nil {
			return resolved, fmt.Errorf("limitrange.limits[%d].max: %w", i, err)
		}
		ri.Min, err = resolveStringMap(r, item.Min)
		if err != nil {
			return resolved, fmt.Errorf("limitrange.limits[%d].min: %w", i, err)
		}
		ri.Default, err = resolveStringMap(r, item.Default)
		if err != nil {
			return resolved, fmt.Errorf("limitrange.limits[%d].default: %w", i, err)
		}
		ri.DefaultRequest, err = resolveStringMap(r, item.DefaultRequest)
		if err != nil {
			return resolved, fmt.Errorf("limitrange.limits[%d].defaultRequest: %w", i, err)
		}
		ri.MaxLimitRequestRatio, err = resolveStringMap(r, item.MaxLimitRequestRatio)
		if err != nil {
			return resolved, fmt.Errorf("limitrange.limits[%d].maxLimitRequestRatio: %w", i, err)
		}
		resolved.Limits = append(resolved.Limits, ri)
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
				return resolved, fmt.Errorf("limitrange.toNamespaces[%d]: %w", i, e)
			}
			if rv != "" {
				resolved.ToNamespaces = append(resolved.ToNamespaces, rv)
			}
		}
	}

	return resolved, nil
}

// HPASpec builds a hpas.Spec from an HPATemplateSource.
// All template expressions must be evaluated before calling here.
func HPASpec(src types.HPATemplateSource, owner domain.Object, reg *types.PresetRegistry) hpas.Spec {
	ownerName := owner.GetName()
	spec := hpas.Spec{
		Name:           src.Name,
		Namespace:      src.Namespace,
		ScaleTargetRef: src.ScaleTargetRef,
		MinReplicas:    1,
		MaxReplicas:    1,
		Labels:         make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-hpa"
	}
	if spec.ScaleTargetRef.Name == "" {
		spec.ScaleTargetRef.Name = ownerName
	}

	if src.MinReplicas != "" {
		if v, err := strconv.ParseInt(src.MinReplicas, 10, 32); err == nil {
			spec.MinReplicas = int32(v)
		}
	}
	if src.MaxReplicas != "" {
		if v, err := strconv.ParseInt(src.MaxReplicas, 10, 32); err == nil {
			spec.MaxReplicas = int32(v)
		}
	}
	if src.TargetCPUUtilizationPercentage != "" {
		if v, err := strconv.ParseInt(src.TargetCPUUtilizationPercentage, 10, 32); err == nil {
			spec.TargetCPUUtilizationPercentage = int32(v)
		}
	}

	if src.Behavior != nil && src.Behavior.Preset != "" {
		if p, ok := reg.All().HPA(src.Behavior.Preset); !ok {
			logger.Warn().Str("preset", src.Behavior.Preset).Msg("unknown hpa behavior preset — skipping")
		} else {
			b := types.HPABehavior{}
			if p.Behavior != nil {
				b = *p.Behavior
			}
			spec.Behavior = &b
			if spec.TargetCPUUtilizationPercentage == 0 {
				spec.TargetCPUUtilizationPercentage = p.CPUTarget()
			}
		}
	} else if src.Behavior != nil {
		b := *src.Behavior
		spec.Behavior = &b
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	// System labels

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// LimitRangeSpec builds a limitranges.Spec from a LimitRangeTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func LimitRangeSpec(src types.LimitRangeTemplateSource, owner domain.Object, reg *types.PresetRegistry) limitranges.Spec {
	limits := src.Limits
	if src.Preset != "" && len(limits) == 0 {
		if p, ok := reg.All().LimitRange(src.Preset); ok {
			limits = p.Limits
		}
	}

	spec := limitranges.Spec{
		Name:           src.Name,
		Namespace:      src.Namespace,
		Limits:         limits,
		FromLimitRange: src.FromLimitRange,
		FromNamespace:  src.FromNamespace,
		Labels:         make(map[string]string),
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// PDBSpec builds a pdbs.Spec from a PDBTemplateSource.
// All template expressions must be evaluated before calling here.
func PDBSpec(src types.PDBTemplateSource, owner domain.Object, reg *types.PresetRegistry) pdbs.Spec {
	ownerName := owner.GetName()
	spec := pdbs.Spec{
		Name:           src.Name,
		Namespace:      src.Namespace,
		MinAvailable:   src.MinAvailable,
		MaxUnavailable: src.MaxUnavailable,
		Selector:       make(map[string]string),
		Labels:         make(map[string]string),
	}

	if spec.Name == "" {
		spec.Name = ownerName + "-pdb"
	}

	if src.Behavior != nil && src.Behavior.Preset != "" {
		if p, ok := reg.All().PDB(src.Behavior.Preset); !ok {
			logger.Warn().Str("preset", src.Behavior.Preset).Msg("unknown pdb behavior preset — skipping")
		} else {
			if spec.MinAvailable == "" {
				spec.MinAvailable = p.MinAvailable
			}
			if spec.MaxUnavailable == "" {
				spec.MaxUnavailable = p.MaxUnavailable
			}
		}
	}

	for k, v := range src.Selector {
		spec.Selector[k] = v
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	// System labels

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}

// ResourceQuotaSpec builds a resourcequotas.Spec from a ResourceQuotaTemplateSource.
// Template expressions must already be evaluated by template.Resolver before calling.
func ResourceQuotaSpec(src types.ResourceQuotaTemplateSource, owner domain.Object, reg *types.PresetRegistry) resourcequotas.Spec {
	hard := src.Hard
	if src.Preset != "" {
		if p, ok := reg.All().ResourceQuota(src.Preset); !ok {
			logger.Warn().Str("preset", src.Preset).Msg("unknown resourcequota preset — skipping")
		} else {
			hard = p.Hard
		}
	}

	spec := resourcequotas.Spec{
		Name:              src.Name,
		Namespace:         src.Namespace,
		Hard:              hard,
		FromResourceQuota: src.FromResourceQuota,
		FromNamespace:     src.FromNamespace,
		Labels:            make(map[string]string),
	}

	for k, v := range src.Labels {
		spec.Labels[k] = v
	}

	spec.Labels = labels.StampInrunLabels(spec.Labels, owner.GetName(), owner.GetAnnotations())
	return spec
}
