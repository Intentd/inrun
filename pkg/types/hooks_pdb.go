package types

// PDBPresetEntry describes a single behavior.preset reference found in a
// PDBTemplateSource. Used by blueprint validation to fail fast on unknown presets
// and to enforce mutual exclusivity with explicit minAvailable/maxUnavailable.
type PDBPresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // PDB name template (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside explicit minAvailable or maxUnavailable
}

type pdbPresetd interface {
	GetPDBBehavior() *PDBBehavior
}

func (t PDBTemplateSource) GetPDBBehavior() *PDBBehavior { return t.Behavior }

// CollectPDBPresetEntries returns all behavior.preset references declared for
// this CRD across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectPDBPresetEntries() []PDBPresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []PDBPresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			pp, ok := res.(pdbPresetd)
			if !ok {
				return
			}
			b := pp.GetPDBBehavior()
			if b == nil || b.Preset == "" {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			mixed := b.MinAvailable != "" || b.MaxUnavailable != ""

			out = append(out, PDBPresetEntry{
				Phase:        phase,
				ResourceName: rname,
				Preset:       b.Preset,
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
