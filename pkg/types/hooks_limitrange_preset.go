package types

// LimitRangePresetEntry describes a single preset reference found in a
// LimitRangeTemplateSource. Used by blueprint validation to fail fast on unknown
// presets and to enforce mutual exclusivity with explicit limits.
type LimitRangePresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // LimitRange name template (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside an explicit Limits list
}

type limitRangePresetd interface {
	getLimitRangePreset() string
	limitRangePresetMixed() bool
}

func (t LimitRangeTemplateSource) getLimitRangePreset() string { return t.Preset }
func (t LimitRangeTemplateSource) limitRangePresetMixed() bool { return len(t.Limits) > 0 }

// CollectLimitRangePresetEntries returns all preset references declared for
// this CRD's limitRanges across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectLimitRangePresetEntries() []LimitRangePresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []LimitRangePresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			rp, ok := res.(limitRangePresetd)
			if !ok {
				return
			}
			preset := rp.getLimitRangePreset()
			if preset == "" {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			out = append(out, LimitRangePresetEntry{
				Phase:        phase,
				ResourceName: rname,
				Preset:       preset,
				Mixed:        rp.limitRangePresetMixed(),
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
