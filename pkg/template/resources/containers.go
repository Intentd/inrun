package resources

import (
	"fmt"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// resolveResourceRequirements resolves template expressions in resources.preset.
// All other ResourceRequirements fields (Requests, Limits) are static k8s quantity strings — not templates.
func resolveResourceRequirements(r *template.Resolver, src *types.ResourceRequirements) (*types.ResourceRequirements, error) {
	if src == nil {
		return nil, nil
	}
	out := *src
	var err error
	if out.Preset, err = r.Resolve(src.Preset); err != nil {
		return nil, fmt.Errorf("resources.preset: %w", err)
	}
	return &out, nil
}

func resolveProbeConfig(r *template.Resolver, src *types.ProbeConfig) (*types.ProbeConfig, error) {
	if src == nil {
		return nil, nil
	}
	out := *src
	var err error
	if out.Path, err = r.Resolve(src.Path); err != nil {
		return nil, fmt.Errorf("probe.path: %w", err)
	}
	return &out, nil
}

func resolveProbes(r *template.Resolver, src *types.ProbesConfig) (*types.ProbesConfig, error) {
	if src == nil {
		return nil, nil
	}
	out := &types.ProbesConfig{}
	var err error
	if out.Startup, err = resolveProbeConfig(r, src.Startup); err != nil {
		return nil, fmt.Errorf("probes.startup: %w", err)
	}
	if out.Liveness, err = resolveProbeConfig(r, src.Liveness); err != nil {
		return nil, fmt.Errorf("probes.liveness: %w", err)
	}
	if out.Readiness, err = resolveProbeConfig(r, src.Readiness); err != nil {
		return nil, fmt.Errorf("probes.readiness: %w", err)
	}
	return out, nil
}

// ── Volume / VolumeMount resolution helpers ───────────────────────────────────

// resolveVolumes resolves template expressions in volume name fields.
// ConfigMap name, Secret name, and PVC claimName all support template expressions.
func resolveVolumes(r *template.Resolver, src []types.VolumeSource) ([]types.VolumeSource, error) {
	if len(src) == 0 {
		return nil, nil
	}
	result := make([]types.VolumeSource, 0, len(src))
	for i, v := range src {
		rv := v
		var err error
		if rv.Name, err = r.Resolve(v.Name); err != nil {
			return nil, fmt.Errorf("volume[%d].name: %w", i, err)
		}
		if v.ConfigMap != nil {
			cm := *v.ConfigMap
			if cm.Name, err = r.Resolve(v.ConfigMap.Name); err != nil {
				return nil, fmt.Errorf("volume[%d].configMap.name: %w", i, err)
			}
			rv.ConfigMap = &cm
		}
		if v.Secret != nil {
			s := *v.Secret
			if s.Name, err = r.Resolve(v.Secret.Name); err != nil {
				return nil, fmt.Errorf("volume[%d].secret.name: %w", i, err)
			}
			rv.Secret = &s
		}
		if v.PersistentVolumeClaim != nil {
			pvc := *v.PersistentVolumeClaim
			if pvc.ClaimName, err = r.Resolve(v.PersistentVolumeClaim.ClaimName); err != nil {
				return nil, fmt.Errorf("volume[%d].persistentVolumeClaim.claimName: %w", i, err)
			}
			rv.PersistentVolumeClaim = &pvc
		}
		result = append(result, rv)
	}
	return result, nil
}

// resolveVolumeMounts resolves template expressions in mount name, mountPath, and subPath.
func resolveVolumeMounts(r *template.Resolver, src []types.VolumeMount) ([]types.VolumeMount, error) {
	if len(src) == 0 {
		return nil, nil
	}
	result := make([]types.VolumeMount, 0, len(src))
	for i, m := range src {
		rm := m
		var err error
		if rm.Name, err = r.Resolve(m.Name); err != nil {
			return nil, fmt.Errorf("volumeMount[%d].name: %w", i, err)
		}
		if rm.MountPath, err = r.Resolve(m.MountPath); err != nil {
			return nil, fmt.Errorf("volumeMount[%d].mountPath: %w", i, err)
		}
		if rm.SubPath, err = r.Resolve(m.SubPath); err != nil {
			return nil, fmt.Errorf("volumeMount[%d].subPath: %w", i, err)
		}
		result = append(result, rm)
	}
	return result, nil
}
