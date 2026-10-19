// pkg/resources/shared/security.go
package shared

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/intentd/inrun/pkg/resources/core/types"
	"github.com/intentd/inrun/pkg/resources/presets"

	"github.com/intentd/inrun/pkg/logger"
)

// ResolveContainerSecurityContext resolves a ContainerSecurityContext:
// if a preset is set it expands to explicit fields; otherwise the block is
// returned as-is. Returns nil when sc is nil.
func ResolveContainerSecurityContext(sc *types.ContainerSecurityContext, reg presets.Registry) *types.ContainerSecurityContext {
	if sc == nil {
		return nil
	}
	if sc.Preset != "" {
		p, ok := reg.ContainerSecurity(sc.Preset)
		if !ok {
			logger.Warn().Str("preset", sc.Preset).Msg("unknown securityContext preset — skipping")
			return nil
		}
		return p.Context()
	}
	return sc
}

// ResolvePodSecurityContext resolves a PodSecurityContext:
// if a preset is set it expands to explicit fields; otherwise the block is
// returned as-is. Returns nil when ps is nil.
func ResolvePodSecurityContext(ps *types.PodSecurityContext, reg presets.Registry) *types.PodSecurityContext {
	if ps == nil {
		return nil
	}
	if ps.Preset != "" {
		p, ok := reg.PodSecurity(ps.Preset)
		if !ok {
			logger.Warn().Str("preset", ps.Preset).Msg("unknown podSecurity preset — skipping")
			return nil
		}
		return p.Context()
	}
	return ps
}

// BuildContainerSecurityContext converts an types.ContainerSecurityContext to
// a Kubernetes corev1.SecurityContext. Returns nil when sc is nil.
func BuildContainerSecurityContext(sc *types.ContainerSecurityContext) *corev1.SecurityContext {
	if sc == nil {
		return nil
	}
	k8s := &corev1.SecurityContext{
		AllowPrivilegeEscalation: sc.AllowPrivilegeEscalation,
		ReadOnlyRootFilesystem:   sc.ReadOnlyRootFilesystem,
		RunAsNonRoot:             sc.RunAsNonRoot,
		RunAsUser:                sc.RunAsUser,
		RunAsGroup:               sc.RunAsGroup,
	}
	if sc.Capabilities != nil {
		k8s.Capabilities = &corev1.Capabilities{}
		for _, cap := range sc.Capabilities.Add {
			k8s.Capabilities.Add = append(k8s.Capabilities.Add, corev1.Capability(cap))
		}
		for _, cap := range sc.Capabilities.Drop {
			k8s.Capabilities.Drop = append(k8s.Capabilities.Drop, corev1.Capability(cap))
		}
	}
	return k8s
}

// BuildPodSecurityContext converts an types.PodSecurityContext to a
// Kubernetes corev1.PodSecurityContext. Returns nil when ps is nil.
func BuildPodSecurityContext(ps *types.PodSecurityContext) *corev1.PodSecurityContext {
	if ps == nil {
		return nil
	}
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ps.RunAsNonRoot,
		RunAsUser:    ps.RunAsUser,
		RunAsGroup:   ps.RunAsGroup,
		FSGroup:      ps.FSGroup,
	}
}

// ApplySecurityContext sets the container security context and pod security context
// on the given container and pod spec. No-op when both are nil.
func ApplySecurityContext(container *corev1.Container, pod *corev1.PodSpec, sc *types.ContainerSecurityContext, ps *types.PodSecurityContext) {
	if sc != nil {
		container.SecurityContext = BuildContainerSecurityContext(sc)
	}
	if ps != nil {
		pod.SecurityContext = BuildPodSecurityContext(ps)
	}
}
