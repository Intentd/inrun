package presets

import (
	"fmt"

	"github.com/intentd/inrun/pkg/presets"
	"github.com/intentd/inrun/pkg/types"
)

// validateUserPresets checks the presets: block declared on the blueprint.
//
// Enforces:
//  1. No duplicate names within a class.
//  2. Warns (does not error) when a user preset shadows a built-in name.
//  3. Each preset entry must have a non-empty name.
func (c *checker) validateUserPresets() error {
	reg := c.Blueprint.Presets
	if reg.Empty() {
		return nil
	}

	type check struct {
		class     string
		names     []string
		isBuiltin func(string) bool
	}

	checks := []check{
		{
			class:     "networkPolicies",
			names:     npDefNames(reg.NetworkPolicies),
			isBuiltin: isBuiltinNetworkPolicy,
		},
		{
			class:     "resourceQuotas",
			names:     rqDefNames(reg.ResourceQuotas),
			isBuiltin: isBuiltinResourceQuota,
		},
		{
			class:     "limitRanges",
			names:     lrDefNames(reg.LimitRanges),
			isBuiltin: nil,
		},
		{
			class:     "hpa",
			names:     hpaDefNames(reg.HPAs),
			isBuiltin: isBuiltinHPA,
		},
		{
			class:     "pdb",
			names:     pdbDefNames(reg.PDBs),
			isBuiltin: isBuiltinPDB,
		},
		{
			class:     "rollingUpdate",
			names:     ruDefNames(reg.RollingUpdates),
			isBuiltin: isBuiltinRollingUpdate,
		},
		{
			class:     "reconciler",
			names:     reconcilerDefNames(reg.Reconciler),
			isBuiltin: presets.IsValidReconcilerPreset,
		},
		{
			class:     "resources",
			names:     resourceDefNames(reg.Resources),
			isBuiltin: isBuiltinResource,
		},
		{
			class:     "probes",
			names:     probeDefNames(reg.Probes),
			isBuiltin: isBuiltinProbe,
		},
		{
			class:     "containerSecurity",
			names:     containerSecurityDefNames(reg.ContainerSecurities),
			isBuiltin: isBuiltinSecurity,
		},
		{
			class:     "podSecurity",
			names:     podSecurityDefNames(reg.PodSecurities),
			isBuiltin: isBuiltinSecurity,
		},
	}

	for _, class := range checks {
		seen := make(map[string]bool, len(class.names))
		for _, name := range class.names {
			if name == "" {
				return fmt.Errorf("%s presets.%s: preset entry is missing a name", failureMark(), class.class)
			}
			if seen[name] {
				return fmt.Errorf("%s presets.%s: duplicate preset name %q — names must be unique within a class", failureMark(), class.class, name)
			}
			seen[name] = true
			if class.isBuiltin != nil && class.isBuiltin(name) {
				warning := fmt.Sprintf("presets.%s %q shadows a built-in Inrun preset — the user-defined version will be used instead",
					class.class, name)
				c.Blueprint.Warnings.AddWarning(warning)
			}
		}
	}
	return nil
}

// isUserNetworkPolicyPreset reports whether name is in the blueprint's user registry.
func (c *checker) isUserNetworkPolicyPreset(name string) bool {
	_, found := c.Blueprint.Presets.NetworkPolicy(name)
	return found
}
func (c *checker) isUserResourceQuotaPreset(name string) bool {
	_, found := c.Blueprint.Presets.ResourceQuota(name)
	return found
}
func (c *checker) isUserLimitRangePreset(name string) bool {
	_, found := c.Blueprint.Presets.LimitRange(name)
	return found
}
func (c *checker) isUserHPAPreset(name string) bool {
	_, found := c.Blueprint.Presets.HPA(name)
	return found
}
func (c *checker) isUserPDBPreset(name string) bool {
	_, found := c.Blueprint.Presets.PDB(name)
	return found
}
func (c *checker) isUserRollingUpdatePreset(name string) bool {
	_, found := c.Blueprint.Presets.RollingUpdate(name)
	return found
}
func npDefNames(defs []types.NetworkPolicyPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func rqDefNames(defs []types.ResourceQuotaPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func lrDefNames(defs []types.LimitRangePresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func hpaDefNames(defs []types.HPAPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func pdbDefNames(defs []types.PDBPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func ruDefNames(defs []types.RollingUpdatePresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func reconcilerDefNames(defs []types.ReconcilerPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func resourceDefNames(defs []types.ResourcePresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func probeDefNames(defs []types.ProbePresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func containerSecurityDefNames(defs []types.ContainerSecurityPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func podSecurityDefNames(defs []types.PodSecurityPresetDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
