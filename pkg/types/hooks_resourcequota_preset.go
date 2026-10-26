package types

// ResourceQuotaPresetEntry describes a single preset reference found in a
// ResourceQuotaTemplateSource. Used by blueprint validation to fail fast on unknown
// presets and to enforce mutual exclusivity with explicit hard limits.
type ResourceQuotaPresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // ResourceQuota name template (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside an explicit hard map
}

type resourceQuotaPreseted interface {
	getResourceQuotaPreset() string
	resourceQuotaPresetMixed() bool
}

func (t ResourceQuotaTemplateSource) getResourceQuotaPreset() string { return t.Preset }
func (t ResourceQuotaTemplateSource) resourceQuotaPresetMixed() bool { return len(t.Hard) > 0 }

// CollectResourceQuotaPresetEntries returns all preset references declared for
// this CRD's resourceQuotas across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectResourceQuotaPresetEntries() []ResourceQuotaPresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []ResourceQuotaPresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			rp, ok := res.(resourceQuotaPreseted)
			if !ok {
				return
			}
			preset := rp.getResourceQuotaPreset()
			if preset == "" {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			out = append(out, ResourceQuotaPresetEntry{
				Phase:        phase,
				ResourceName: rname,
				Preset:       preset,
				Mixed:        rp.resourceQuotaPresetMixed(),
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
