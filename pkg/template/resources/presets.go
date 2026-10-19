package resources

import (
	"github.com/intentd/inrun/pkg/logger"
	restypes "github.com/intentd/kuberesources/core/types"
	"github.com/intentd/kuberesources/presets"
)

// presetResources fills resource requirements from their preset; an unknown
// preset is logged and the field left unset.
func presetResources(r *restypes.ResourceRequirements, reg presets.Registry) *restypes.ResourceRequirements {
	out, err := presets.ExpandResources(r, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping resources")
	}
	return out
}

// presetContainerSecurity fills a container security context from its preset;
// an unknown preset is logged and the field left unset.
func presetContainerSecurity(sc *restypes.ContainerSecurityContext, reg presets.Registry) *restypes.ContainerSecurityContext {
	out, err := presets.ExpandContainerSecurity(sc, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping securityContext")
	}
	return out
}

// presetPodSecurity fills a pod security context from its preset; an unknown
// preset is logged and the field left unset.
func presetPodSecurity(ps *restypes.PodSecurityContext, reg presets.Registry) *restypes.PodSecurityContext {
	out, err := presets.ExpandPodSecurity(ps, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping podSecurity")
	}
	return out
}
