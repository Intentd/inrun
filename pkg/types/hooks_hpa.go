package types

import (
	"fmt"
	"strings"
)

// HPAPresetEntry describes a single behavior.preset reference found in an
// HPATemplateSource. Used by blueprint validation to fail fast on unknown presets
// and to enforce mutual exclusivity with explicit scaleUp/scaleDown fields.
type HPAPresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // HPA name template (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside explicit scaleUp or scaleDown
}

// hpaPresetd is satisfied by HPATemplateSource.
type hpaPresetd interface {
	GetHPABehavior() *HPABehavior
}

func (t HPATemplateSource) GetHPABehavior() *HPABehavior { return t.Behavior }

// CollectHPAPresetEntries returns all behavior.preset references declared for
// this CRD across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectHPAPresetEntries() []HPAPresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []HPAPresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			hp, ok := res.(hpaPresetd)
			if !ok {
				return
			}
			b := hp.GetHPABehavior()
			if b == nil || b.Preset == "" {
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
			_ = rtype

			mixed := b.ScaleUp != nil || b.ScaleDown != nil

			out = append(out, HPAPresetEntry{
				Phase:        phase,
				ResourceName: rname,
				Preset:       b.Preset,
				Mixed:        mixed,
			})
		})
	}

	if c.HasOnCreate() {
		collect("onCreate", c.Box().EffectiveOnCreate())
	}
	if c.HasOnReconcile() {
		collect("onReconcile", c.Box().EffectiveOnReconcile())
	}
	if c.HasOnDelete() {
		collect("onDelete", c.Box().EffectiveOnDelete())
	}

	return out
}
