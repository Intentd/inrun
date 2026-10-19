package types

import (
	"fmt"
	"strings"
)

// ProbePresetEntry describes a single probe preset reference found in a
// HookTemplates resource. Used by validation to fail fast on unknown presets
// and to enforce mutual exclusivity between preset and explicit timing fields.
type ProbePresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	Resource     string // e.g. "Deployment", "StatefulSet"
	ResourceName string // template name field (may be Empty()
	ProbeType    string // "startup", "liveness", "readiness"
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside explicit timing overrides
}

// prober is a tiny interface satisfied by resources that carry a ProbesConfig.
type prober interface {
	GetProbes() *ProbesConfig
}

// GetProbes returns the probe configuration for each supported workload type.
// Value receivers so the method is available on both values and pointers.
func (t DeploymentTemplateSource) GetProbes() *ProbesConfig  { return t.Probes }
func (t ReplicaSetTemplateSource) GetProbes() *ProbesConfig  { return t.Probes }
func (t StatefulSetTemplateSource) GetProbes() *ProbesConfig { return t.Probes }
func (t PodTemplateSource) GetProbes() *ProbesConfig         { return t.Probes }

// CollectProbePresetEntries returns all probe preset references declared for
// this CRD across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned — omitted presets
// (which fall back to "standard") are not surfaced for validation.
func (c *CRDEntry) CollectProbePresetEntries() []ProbePresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []ProbePresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			pb, ok := res.(prober)
			if !ok {
				return
			}
			probes := pb.GetProbes()
			if probes == nil {
				return
			}

			// Derive friendly names
			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}
			rtype := fmt.Sprintf("%T", res)
			if i := strings.LastIndex(rtype, "."); i >= 0 {
				rtype = rtype[i+1:]
			}

			for probeType, cfg := range map[string]*ProbeConfig{
				"startup":   probes.Startup,
				"liveness":  probes.Liveness,
				"readiness": probes.Readiness,
			} {
				if cfg == nil || cfg.Preset == "" {
					continue
				}
				mixed := cfg.InitialDelaySeconds != nil ||
					cfg.PeriodSeconds != nil ||
					cfg.FailureThreshold != nil ||
					cfg.SuccessThreshold != nil ||
					cfg.TimeoutSeconds != nil
				out = append(out, ProbePresetEntry{
					Phase:        phase,
					Resource:     rtype,
					ResourceName: rname,
					ProbeType:    probeType,
					Preset:       cfg.Preset,
					Mixed:        mixed,
				})
			}
		})
	}

	if c.HasOnCreate() {
		collect("onCreate", c.OperatorBox.EffectiveOnCreate())
	}
	if c.HasOnReconcile() {
		collect("onReconcile", c.OperatorBox.EffectiveOnReconcile())
	}
	if c.HasOnDelete() {
		collect("onDelete", c.OperatorBox.EffectiveOnDelete())
	}

	return out
}
