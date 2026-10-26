package resources

// The pod helpers. Every pod builder (pods, deployments, replicaSets,
// statefulSets, jobs, cronJobs) resolves and maps its container fields here,
// as kubewright builds every pod kind's containers in one place, so a
// container field is handled once for all six kinds. TestPodKindsCarryEveryField
// fails when a template field stops reaching kubewright.

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	kwtypes "github.com/intentd/kubewright/core/types"
	"github.com/intentd/kubewright/presets"
)

// resolveEnv resolves env values and the names of the Secrets and ConfigMaps
// they reference.
func resolveEnv(r *template.Resolver, src []types.EnvVar) ([]types.EnvVar, error) {
	if len(src) == 0 {
		return nil, nil
	}
	out := make([]types.EnvVar, 0, len(src))
	for _, v := range src {
		ev := types.EnvVar{Name: v.Name}
		var err error
		if v.Value != "" {
			if ev.Value, err = r.Resolve(v.Value); err != nil {
				return nil, fmt.Errorf("env[%s].value: %w", v.Name, err)
			}
		}
		if v.ValueFrom != nil {
			ev.ValueFrom = &types.ValueFrom{}
			if ref := v.ValueFrom.SecretKeyRef; ref != nil {
				name, err := r.Resolve(ref.Name)
				if err != nil {
					return nil, fmt.Errorf("env[%s].valueFrom.secretKeyRef.name: %w", v.Name, err)
				}
				ev.ValueFrom.SecretKeyRef = &types.SecretKeyRef{Name: name, Key: ref.Key}
			}
			if ref := v.ValueFrom.ConfigMapKeyRef; ref != nil {
				name, err := r.Resolve(ref.Name)
				if err != nil {
					return nil, fmt.Errorf("env[%s].valueFrom.configMapKeyRef.name: %w", v.Name, err)
				}
				ev.ValueFrom.ConfigMapKeyRef = &types.ConfigMapKeyRef{Name: name, Key: ref.Key}
			}
		}
		out = append(out, ev)
	}
	return out, nil
}

// resolveEnvFrom resolves the names of the Secrets and ConfigMaps loaded whole.
func resolveEnvFrom(r *template.Resolver, src *types.EnvFrom) (*types.EnvFrom, error) {
	if src == nil {
		return nil, nil
	}
	out := &types.EnvFrom{}
	var err error
	if out.SecretRef, err = r.ResolveEnvFromRefs(src.SecretRef, "envFrom.secretRef"); err != nil {
		return nil, err
	}
	if out.ConfigMapRef, err = r.ResolveEnvFromRefs(src.ConfigMapRef, "envFrom.configMapRef"); err != nil {
		return nil, err
	}
	return out, nil
}

// resolveContainers resolves the template expressions in each container.
func resolveContainers(r *template.Resolver, src []types.Container, field string) ([]types.Container, error) {
	if len(src) == 0 {
		return nil, nil
	}
	out := make([]types.Container, 0, len(src))
	for i, c := range src {
		rc, err := resolveContainer(r, c)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", field, i, err)
		}
		out = append(out, rc)
	}
	return out, nil
}

func resolveContainer(r *template.Resolver, c types.Container) (types.Container, error) {
	out := c
	var err error
	for _, f := range []struct {
		name string
		dst  *string
	}{
		{"name", &out.Name},
		{"image", &out.Image},
		{"imagePullPolicy", &out.ImagePullPolicy},
		{"port", &out.Port},
		{"protocol", &out.Protocol},
	} {
		if *f.dst, err = r.Resolve(*f.dst); err != nil {
			return out, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	if out.Command, err = r.ResolveArgs(c.Command, "command"); err != nil {
		return out, err
	}
	if out.Args, err = r.ResolveArgs(c.Args, "args"); err != nil {
		return out, err
	}
	if out.Env, err = resolveEnv(r, c.Env); err != nil {
		return out, err
	}
	if out.EnvFrom, err = resolveEnvFrom(r, c.EnvFrom); err != nil {
		return out, err
	}
	if out.Resources, err = resolveResourceRequirements(r, c.Resources); err != nil {
		return out, fmt.Errorf("resources: %w", err)
	}
	if out.Probes, err = resolveProbes(r, c.Probes); err != nil {
		return out, fmt.Errorf("probes: %w", err)
	}
	if out.VolumeMounts, err = resolveVolumeMounts(r, c.VolumeMounts); err != nil {
		return out, err
	}
	return out, nil
}

// containerSpecs maps resolved containers to kubewright's, expanding their
// resources, probes and securityContext presets.
func containerSpecs(src []types.Container, reg presets.Registry) []kwtypes.Container {
	if len(src) == 0 {
		return nil
	}
	out := make([]kwtypes.Container, 0, len(src))
	for _, c := range src {
		k := kwtypes.Container{
			Name:            c.Name,
			Image:           c.Image,
			ImagePullPolicy: corev1.PullPolicy(c.ImagePullPolicy),
			Command:         c.Command,
			Args:            c.Args,
			Env:             []kwtypes.EnvVar(c.Env),
			EnvFrom:         c.EnvFrom,
			Resources:       presetResources(c.Resources, reg),
			Probes:          presetProbes(c.Probes, reg),
			SecurityContext: presetContainerSecurity(c.SecurityContext, reg),
			VolumeMounts:    c.VolumeMounts,
		}
		if c.Port != "" {
			k.Port = int32(parsePort(c.Port))
		}
		if c.Protocol != "" {
			k.Protocol = ParseProtocol(c.Protocol)
		}
		out = append(out, k)
	}
	return out
}
