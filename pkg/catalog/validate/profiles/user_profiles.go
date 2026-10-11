package profiles

import (
	"fmt"

	"github.com/intentd/inrun/pkg/profiles"
	"github.com/intentd/inrun/pkg/types"
)

// validateUserProfiles checks the profiles: block declared on the catalog.
//
// Enforces:
//  1. No duplicate names within a class.
//  2. Warns (does not error) when a user profile shadows a built-in name.
//  3. Each profile entry must have a non-empty name.
func (c *checker) validateUserProfiles() error {
	reg := c.Cat.Profiles
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
			isBuiltin: profiles.IsValidNetworkPolicyProfile,
		},
		{
			class:     "resourceQuotas",
			names:     rqDefNames(reg.ResourceQuotas),
			isBuiltin: profiles.IsValidResourceQuotaProfile,
		},
		{
			class:     "limitRanges",
			names:     lrDefNames(reg.LimitRanges),
			isBuiltin: nil,
		},
		{
			class:     "hpa",
			names:     hpaDefNames(reg.HPA),
			isBuiltin: profiles.IsValidHPAProfile,
		},
		{
			class:     "pdb",
			names:     pdbDefNames(reg.PDB),
			isBuiltin: profiles.IsValidPDBProfile,
		},
		{
			class:     "rollingUpdate",
			names:     ruDefNames(reg.RollingUpdate),
			isBuiltin: profiles.IsValidRollingUpdateProfile,
		},
		{
			class:     "reconciler",
			names:     reconcilerDefNames(reg.Reconciler),
			isBuiltin: profiles.IsValidReconcilerProfile,
		},
		{
			class:     "resources",
			names:     resourceDefNames(reg.Resources),
			isBuiltin: profiles.IsValidResourceProfile,
		},
		{
			class:     "probes",
			names:     probeDefNames(reg.Probes),
			isBuiltin: profiles.IsValidProbeProfile,
		},
		{
			class:     "containerSecurity",
			names:     containerSecurityDefNames(reg.ContainerSecurity),
			isBuiltin: profiles.IsValidSecurityProfile,
		},
		{
			class:     "podSecurity",
			names:     podSecurityDefNames(reg.PodSecurity),
			isBuiltin: profiles.IsValidSecurityProfile,
		},
	}

	for _, class := range checks {
		seen := make(map[string]bool, len(class.names))
		for _, name := range class.names {
			if name == "" {
				return fmt.Errorf("%s profiles.%s: profile entry is missing a name", failureMark(), class.class)
			}
			if seen[name] {
				return fmt.Errorf("%s profiles.%s: duplicate profile name %q — names must be unique within a class", failureMark(), class.class, name)
			}
			seen[name] = true
			if class.isBuiltin != nil && class.isBuiltin(name) {
				warning := fmt.Sprintf("profiles.%s %q shadows a built-in Inrun profile — the user-defined version will be used instead",
					class.class, name)
				c.Cat.Warnings.AddWarning(warning)
			}
		}
	}
	return nil
}

// isUserNetworkPolicyProfile reports whether name is in the catalog's user registry.
func (c *checker) isUserNetworkPolicyProfile(name string) bool {
	_, found := c.Cat.Profiles.LookupNetworkPolicy(name)
	return found
}
func (c *checker) isUserResourceQuotaProfile(name string) bool {
	_, found := c.Cat.Profiles.LookupResourceQuota(name)
	return found
}
func (c *checker) isUserLimitRangeProfile(name string) bool {
	_, found := c.Cat.Profiles.LookupLimitRange(name)
	return found
}
func (c *checker) isUserHPAProfile(name string) bool {
	_, found := c.Cat.Profiles.LookupHPA(name)
	return found
}
func (c *checker) isUserPDBProfile(name string) bool {
	_, found := c.Cat.Profiles.LookupPDB(name)
	return found
}
func (c *checker) isUserRollingUpdateProfile(name string) bool {
	_, found := c.Cat.Profiles.LookupRollingUpdate(name)
	return found
}
func npDefNames(defs []types.NetworkPolicyProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func rqDefNames(defs []types.ResourceQuotaProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func lrDefNames(defs []types.LimitRangeProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func hpaDefNames(defs []types.HPAProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func pdbDefNames(defs []types.PDBProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func ruDefNames(defs []types.RollingUpdateProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func reconcilerDefNames(defs []types.ReconcilerProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func resourceDefNames(defs []types.ResourceProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func probeDefNames(defs []types.ProbeProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func containerSecurityDefNames(defs []types.ContainerSecurityProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
func podSecurityDefNames(defs []types.PodSecurityProfileDef) []string {
	out := make([]string, len(defs))
	for i, d := range defs {
		out[i] = d.Name
	}
	return out
}
