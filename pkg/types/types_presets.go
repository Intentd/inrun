package types

import (
	"fmt"

	"github.com/intentd/kubekit/presets"
)

// PresetRegistry holds all user-defined presets declared in a Blueprint or Module.
// Presets are resolved before built-ins at validate and reconcile time.
// Template expressions in preset field values are allowed and resolved at reconcile time.
type PresetRegistry struct {
	Include string `yaml:"include,omitempty" json:"include,omitempty"`

	// The resource presets, inline: networkPolicies, resources, probes …
	presets.Registry `yaml:",inline"`

	// Reconciler presets tune a CRD's own reconciler, so they stay Inrun's.
	Reconciler []ReconcilerPresetDef `yaml:"reconciler,omitempty" json:"reconciler,omitempty"`
}

// All returns the built-in presets merged with r's, which replace
// built-ins of the same name.
func (r *PresetRegistry) All() presets.Registry {
	if r == nil {
		return presets.Builtin()
	}
	return presets.Builtin().With(r.Registry)
}

func (r PresetRegistry) Empty() bool {
	return r.Registry.Empty() && len(r.Reconciler) == 0
}

func (r PresetRegistry) LookupReconciler(name string) (ReconcilerPresetDef, bool) {
	for _, e := range r.Reconciler {
		if e.Name == name {
			return e, true
		}
	}
	return ReconcilerPresetDef{}, false
}

// Merge combines other into r, returning a conflict error if the same name
// appears in the same class in both registries.
func (r PresetRegistry) Merge(other PresetRegistry, otherSource string) (PresetRegistry, error) {
	merged := r
	if err := presetConflict(r.Registry, other.Registry, otherSource); err != nil {
		return PresetRegistry{}, err
	}
	merged.Registry = r.Registry.With(other.Registry)
	for _, e := range other.Reconciler {
		if _, found := r.LookupReconciler(e.Name); found {
			return PresetRegistry{}, presetConflictError("reconciler", e.Name, otherSource)
		}
		merged.Reconciler = append(merged.Reconciler, e)
	}
	return merged, nil
}

func presetConflictError(class, name, source string) error {
	return fmt.Errorf("preset conflict: %s %q defined in both %s and the blueprint", class, name, source)
}

// NetworkPolicyPresetDef is presets.NetworkPolicy.
type NetworkPolicyPresetDef = presets.NetworkPolicy

// ResourceQuotaPresetDef is presets.ResourceQuota.
type ResourceQuotaPresetDef = presets.ResourceQuota

// LimitRangePresetDef is presets.LimitRange.
type LimitRangePresetDef = presets.LimitRange

// HPAPresetDef is presets.HPA.
type HPAPresetDef = presets.HPA

// PDBPresetDef is presets.PDB.
type PDBPresetDef = presets.PDB

// RollingUpdatePresetDef is presets.RollingUpdate.
type RollingUpdatePresetDef = presets.RollingUpdate

// ReconcilerPresetDef defines a named reconciler tuning preset.
// All fields mirror the inline fields under operatorBox.reconciler.
// Built-in presets: high-throughput, conservative, development.
type ReconcilerPresetDef struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Workers     int      `yaml:"workers,omitempty" json:"workers,omitempty"`
	Resync      Duration `yaml:"resync,omitempty" json:"resync,omitempty"`
	Queue       Queue    `yaml:"queue,omitempty" json:"queue,omitempty"`
}

// ResourcePresetDef is presets.Resource.
type ResourcePresetDef = presets.Resource

// ProbePresetDef is presets.Probe.
type ProbePresetDef = presets.Probe

// ContainerSecurityPresetDef is presets.ContainerSecurity.
type ContainerSecurityPresetDef = presets.ContainerSecurity

// PodSecurityPresetDef is presets.PodSecurity.
type PodSecurityPresetDef = presets.PodSecurity

// presetConflict reports the first preset other declares that r already has:
// a Blueprint and a module it imports may not define the same preset.
func presetConflict(r, other presets.Registry, otherSource string) error {
	check := func(class string, names []string, has func(string) bool) error {
		for _, n := range names {
			if has(n) {
				return presetConflictError(class, n, otherSource)
			}
		}
		return nil
	}
	for _, c := range []struct {
		class string
		names []string
		has   func(string) bool
	}{
		{"networkPolicies", presetNames(other.NetworkPolicies, func(p presets.NetworkPolicy) string { return p.Name }), func(n string) bool { _, ok := r.NetworkPolicy(n); return ok }},
		{"resourceQuotas", presetNames(other.ResourceQuotas, func(p presets.ResourceQuota) string { return p.Name }), func(n string) bool { _, ok := r.ResourceQuota(n); return ok }},
		{"limitRanges", presetNames(other.LimitRanges, func(p presets.LimitRange) string { return p.Name }), func(n string) bool { _, ok := r.LimitRange(n); return ok }},
		{"hpa", presetNames(other.HPAs, func(p presets.HPA) string { return p.Name }), func(n string) bool { _, ok := r.HPA(n); return ok }},
		{"pdb", presetNames(other.PDBs, func(p presets.PDB) string { return p.Name }), func(n string) bool { _, ok := r.PDB(n); return ok }},
		{"rollingUpdate", presetNames(other.RollingUpdates, func(p presets.RollingUpdate) string { return p.Name }), func(n string) bool { _, ok := r.RollingUpdate(n); return ok }},
		{"resources", presetNames(other.Resources, func(p presets.Resource) string { return p.Name }), func(n string) bool { _, ok := r.Resource(n); return ok }},
		{"probes", presetNames(other.Probes, func(p presets.Probe) string { return p.Name }), func(n string) bool { _, ok := r.Probe(n); return ok }},
		{"containerSecurity", presetNames(other.ContainerSecurities, func(p presets.ContainerSecurity) string { return p.Name }), func(n string) bool { _, ok := r.ContainerSecurity(n); return ok }},
		{"podSecurity", presetNames(other.PodSecurities, func(p presets.PodSecurity) string { return p.Name }), func(n string) bool { _, ok := r.PodSecurity(n); return ok }},
	} {
		if err := check(c.class, c.names, c.has); err != nil {
			return err
		}
	}
	return nil
}

func presetNames[P any](ps []P, name func(P) string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = name(p)
	}
	return out
}
