package resources

import (
	"github.com/intentd/inrun/pkg/logger"
	kwtypes "github.com/intentd/kubewright/core/types"
	"github.com/intentd/kubewright/presets"
)

// presetResources fills resource requirements from their preset; an unknown
// preset is logged and the field left unset.
func presetResources(r *kwtypes.ResourceRequirements, reg presets.Registry) *kwtypes.ResourceRequirements {
	out, err := presets.ExpandResources(r, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping resources")
	}
	return out
}

// presetContainerSecurity fills a container security context from its preset;
// an unknown preset is logged and the field left unset.
func presetContainerSecurity(sc *kwtypes.ContainerSecurityContext, reg presets.Registry) *kwtypes.ContainerSecurityContext {
	out, err := presets.ExpandContainerSecurity(sc, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping securityContext")
	}
	return out
}

// presetPodSecurity fills a pod security context from its preset; an unknown
// preset is logged and the field left unset.
func presetPodSecurity(ps *kwtypes.PodSecurityContext, reg presets.Registry) *kwtypes.PodSecurityContext {
	out, err := presets.ExpandPodSecurity(ps, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping podSecurity")
	}
	return out
}

// presetRollingUpdate fills a rolling update from its preset; an unknown
// preset is logged and the field left unset.
func presetRollingUpdate(r *kwtypes.RollingUpdateBehavior, reg presets.Registry) *kwtypes.RollingUpdateBehavior {
	out, err := presets.ExpandRollingUpdate(r, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping rollingUpdate")
	}
	return out
}

// presetHPA fills an HPA's behavior and CPU target from its preset; an
// unknown preset is logged and the behavior left unset.
func presetHPA(b *kwtypes.HPABehavior, cpuTarget int32, reg presets.Registry) (*kwtypes.HPABehavior, int32) {
	out, target, err := presets.ExpandHPA(b, cpuTarget, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping hpa behavior")
	}
	return out, target
}

// presetPDB fills a PDB's unset limits from its preset; an unknown preset is
// logged and the limits kept.
func presetPDB(name, minAvailable, maxUnavailable string, reg presets.Registry) (string, string) {
	minA, maxU, err := presets.ExpandPDB(name, minAvailable, maxUnavailable, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping pdb preset")
	}
	return minA, maxU
}

// presetNetworkPolicy replaces the rules with their preset; an unknown preset
// is logged and the rules kept.
func presetNetworkPolicy(name string, rules presets.NetworkPolicyRules, reg presets.Registry) presets.NetworkPolicyRules {
	out, err := presets.ExpandNetworkPolicy(name, rules, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping networkPolicy preset")
	}
	return out
}

// presetResourceQuota replaces the hard limits with their preset; an unknown
// preset is logged and the limits kept.
func presetResourceQuota(name string, hard map[string]string, reg presets.Registry) map[string]string {
	out, err := presets.ExpandResourceQuota(name, hard, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping resourceQuota preset")
	}
	return out
}

// presetLimitRange fills empty limits from their preset; an unknown preset is
// logged and the limits kept.
func presetLimitRange(name string, limits []kwtypes.LimitRangeItem, reg presets.Registry) []kwtypes.LimitRangeItem {
	out, err := presets.ExpandLimitRange(name, limits, reg)
	if err != nil {
		logger.Warn().Err(err).Msg("skipping limitRange preset")
	}
	return out
}
