package presets

import (
	"fmt"

	"github.com/intentd/inrun/pkg/resources/core/types"
)

// ExpandResources resolves resource requirements: if resources.preset is set
// it expands to explicit requests/limits; otherwise the block is returned as-is.
// Returns nil when neither preset nor explicit values are declared, and an
// error for an unknown preset.
func ExpandResources(r *types.ResourceRequirements, reg Registry) (*types.ResourceRequirements, error) {
	if r == nil {
		return nil, nil
	}
	if r.Preset != "" {
		p, ok := reg.Resource(r.Preset)
		if !ok {
			return nil, fmt.Errorf("unknown resources preset %q", r.Preset)
		}
		return p.Requirements(), nil
	}
	if len(r.Requests) == 0 && len(r.Limits) == 0 {
		return nil, nil
	}
	return r, nil
}

// ExpandContainerSecurity resolves a ContainerSecurityContext:
// if a preset is set it expands to explicit fields; otherwise the block is
// returned as-is; an unknown preset is an error. Returns nil when sc is nil.
func ExpandContainerSecurity(sc *types.ContainerSecurityContext, reg Registry) (*types.ContainerSecurityContext, error) {
	if sc == nil {
		return nil, nil
	}
	if sc.Preset != "" {
		p, ok := reg.ContainerSecurity(sc.Preset)
		if !ok {
			return nil, fmt.Errorf("unknown securityContext preset %q", sc.Preset)
		}
		return p.Context(), nil
	}
	return sc, nil
}

// ExpandPodSecurity resolves a PodSecurityContext:
// if a preset is set it expands to explicit fields; otherwise the block is
// returned as-is; an unknown preset is an error. Returns nil when ps is nil.
func ExpandPodSecurity(ps *types.PodSecurityContext, reg Registry) (*types.PodSecurityContext, error) {
	if ps == nil {
		return nil, nil
	}
	if ps.Preset != "" {
		p, ok := reg.PodSecurity(ps.Preset)
		if !ok {
			return nil, fmt.Errorf("unknown podSecurity preset %q", ps.Preset)
		}
		return p.Context(), nil
	}
	return ps, nil
}
