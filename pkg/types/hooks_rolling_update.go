package types

// RollingUpdatePresetEntry describes a single rollingUpdate.preset reference found in a
// DeploymentTemplateSource. Used by blueprint validation to fail fast on unknown presets
// and to enforce mutual exclusivity with explicit maxSurge/maxUnavailable.
type RollingUpdatePresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // Deployment name template (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside explicit maxSurge or maxUnavailable
}

type rollingUpdatePresetd interface {
	GetRollingUpdate() *RollingUpdateBehavior
}

func (t DeploymentTemplateSource) GetRollingUpdate() *RollingUpdateBehavior  { return t.RollingUpdate }
func (t StatefulSetTemplateSource) GetRollingUpdate() *RollingUpdateBehavior { return t.RollingUpdate }
func (t ReplicaSetTemplateSource) GetRollingUpdate() *RollingUpdateBehavior  { return t.RollingUpdate }

// CollectRollingUpdatePresetEntries returns all rollingUpdate.preset references declared
// for this CRD across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectRollingUpdatePresetEntries() []RollingUpdatePresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []RollingUpdatePresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			rp, ok := res.(rollingUpdatePresetd)
			if !ok {
				return
			}
			b := rp.GetRollingUpdate()
			if b == nil || b.Preset == "" {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			mixed := b.MaxSurge != "" || b.MaxUnavailable != ""

			out = append(out, RollingUpdatePresetEntry{
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
