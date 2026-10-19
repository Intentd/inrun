package types

import (
	"fmt"
	"strings"
)

// ResourcePresetEntry describes a single resources.preset reference found in a
// HookTemplates resource. Used by validation to fail fast on unknown presets
// and to enforce mutual exclusivity between preset and explicit resource fields.
type ResourcePresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	Resource     string // e.g. "Deployment", "StatefulSet"
	ResourceName string // template name field (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside explicit requests/limits
}

// resourced is a tiny interface satisfied by resources that carry a ResourceRequirements block.
type resourced interface {
	GetResources() *ResourceRequirements
}

// GetResources returns the resource requirements for each supported workload type.
func (t DeploymentTemplateSource) GetResources() *ResourceRequirements  { return t.Resources }
func (t ReplicaSetTemplateSource) GetResources() *ResourceRequirements  { return t.Resources }
func (t StatefulSetTemplateSource) GetResources() *ResourceRequirements { return t.Resources }
func (t PodTemplateSource) GetResources() *ResourceRequirements         { return t.Resources }
func (t JobTemplateSource) GetResources() *ResourceRequirements         { return t.Resources }
func (t CronJobTemplateSource) GetResources() *ResourceRequirements     { return t.Resources }

// CollectResourcePresetEntries returns all resources.preset references declared for
// this CRD across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectResourcePresetEntries() []ResourcePresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []ResourcePresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			rd, ok := res.(resourced)
			if !ok {
				return
			}
			spec := rd.GetResources()
			if spec == nil || spec.Preset == "" {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}
			rtype := fmt.Sprintf("%T", res)
			if i := strings.LastIndex(rtype, "."); i >= 0 {
				rtype = rtype[i+1:]
			}

			mixed := len(spec.Requests) > 0 || len(spec.Limits) > 0

			out = append(out, ResourcePresetEntry{
				Phase:        phase,
				Resource:     rtype,
				ResourceName: rname,
				Preset:       spec.Preset,
				Mixed:        mixed,
			})
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
