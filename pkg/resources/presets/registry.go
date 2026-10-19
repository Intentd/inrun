package presets

import (
	"slices"
	"strings"
)

// Registry is a set of named presets, one list per kind of field. Builtin
// returns the built-in presets; a program that declares its own merges them
// with With.
type Registry struct {
	Resources           []Resource          `yaml:"resources,omitempty" json:"resources,omitempty"`
	Probes              []Probe             `yaml:"probes,omitempty" json:"probes,omitempty"`
	ContainerSecurities []ContainerSecurity `yaml:"containerSecurity,omitempty" json:"containerSecurity,omitempty"`
	PodSecurities       []PodSecurity       `yaml:"podSecurity,omitempty" json:"podSecurity,omitempty"`
	RollingUpdates      []RollingUpdate     `yaml:"rollingUpdate,omitempty" json:"rollingUpdate,omitempty"`
	HPAs                []HPA               `yaml:"hpa,omitempty" json:"hpa,omitempty"`
	PDBs                []PDB               `yaml:"pdb,omitempty" json:"pdb,omitempty"`
	NetworkPolicies     []NetworkPolicy     `yaml:"networkPolicies,omitempty" json:"networkPolicies,omitempty"`
	ResourceQuotas      []ResourceQuota     `yaml:"resourceQuotas,omitempty" json:"resourceQuotas,omitempty"`
	LimitRanges         []LimitRange        `yaml:"limitRanges,omitempty" json:"limitRanges,omitempty"`
}

// Empty reports whether r has no presets.
func (r Registry) Empty() bool {
	return len(r.Resources) == 0 &&
		len(r.Probes) == 0 &&
		len(r.ContainerSecurities) == 0 &&
		len(r.PodSecurities) == 0 &&
		len(r.RollingUpdates) == 0 &&
		len(r.HPAs) == 0 &&
		len(r.PDBs) == 0 &&
		len(r.NetworkPolicies) == 0 &&
		len(r.ResourceQuotas) == 0 &&
		len(r.LimitRanges) == 0
}

// With returns r merged with other. A preset in other replaces the one in r
// with the same name (case-insensitive).
func (r Registry) With(other Registry) Registry {
	return Registry{
		Resources:           with(r.Resources, other.Resources),
		Probes:              with(r.Probes, other.Probes),
		ContainerSecurities: with(r.ContainerSecurities, other.ContainerSecurities),
		PodSecurities:       with(r.PodSecurities, other.PodSecurities),
		RollingUpdates:      with(r.RollingUpdates, other.RollingUpdates),
		HPAs:                with(r.HPAs, other.HPAs),
		PDBs:                with(r.PDBs, other.PDBs),
		NetworkPolicies:     with(r.NetworkPolicies, other.NetworkPolicies),
		ResourceQuotas:      with(r.ResourceQuotas, other.ResourceQuotas),
		LimitRanges:         with(r.LimitRanges, other.LimitRanges),
	}
}

// Resource returns the resource preset called name (case-insensitive).
func (r Registry) Resource(name string) (Resource, bool) { return get(r.Resources, name) }

// Probe returns the probe preset called name (case-insensitive).
func (r Registry) Probe(name string) (Probe, bool) { return get(r.Probes, name) }

// ContainerSecurity returns the container security preset called name (case-insensitive).
func (r Registry) ContainerSecurity(name string) (ContainerSecurity, bool) {
	return get(r.ContainerSecurities, name)
}

// PodSecurity returns the pod security preset called name (case-insensitive).
func (r Registry) PodSecurity(name string) (PodSecurity, bool) { return get(r.PodSecurities, name) }

// RollingUpdate returns the rolling update preset called name (case-insensitive).
func (r Registry) RollingUpdate(name string) (RollingUpdate, bool) {
	return get(r.RollingUpdates, name)
}

// HPA returns the HPA preset called name (case-insensitive).
func (r Registry) HPA(name string) (HPA, bool) { return get(r.HPAs, name) }

// PDB returns the PDB preset called name (case-insensitive).
func (r Registry) PDB(name string) (PDB, bool) { return get(r.PDBs, name) }

// NetworkPolicy returns the network policy preset called name (case-insensitive).
func (r Registry) NetworkPolicy(name string) (NetworkPolicy, bool) {
	return get(r.NetworkPolicies, name)
}

// ResourceQuota returns the resource quota preset called name (case-insensitive).
func (r Registry) ResourceQuota(name string) (ResourceQuota, bool) {
	return get(r.ResourceQuotas, name)
}

// LimitRange returns the limit range preset called name (case-insensitive).
func (r Registry) LimitRange(name string) (LimitRange, bool) { return get(r.LimitRanges, name) }

type named interface{ presetName() string }

func get[P named](presets []P, name string) (P, bool) {
	for _, p := range presets {
		if strings.EqualFold(p.presetName(), name) {
			return p, true
		}
	}
	var zero P
	return zero, false
}

func with[P named](base, over []P) []P {
	if len(over) == 0 {
		return slices.Clone(base)
	}
	out := make([]P, 0, len(base)+len(over))
	for _, p := range base {
		if _, replaced := get(over, p.presetName()); !replaced {
			out = append(out, p)
		}
	}
	return append(out, over...)
}
