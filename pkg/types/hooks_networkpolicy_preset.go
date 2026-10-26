package types

// NetworkPolicyPresetEntry describes a single preset reference found in a
// NetworkPolicyTemplateSource. Used by blueprint validation to fail fast on unknown
// presets and to enforce mutual exclusivity with explicit ingress/egress/policyTypes.
type NetworkPolicyPresetEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // NetworkPolicy name template (may be Empty()
	Preset       string // raw preset name as written in the blueprint
	Mixed        bool   // true when preset is set alongside explicit ingress/egress/policyTypes
}

type networkPolicyPresetd interface {
	getNetworkPolicyPreset() string
	networkPolicyPresetMixed() bool
}

func (t NetworkPolicyTemplateSource) getNetworkPolicyPreset() string { return t.Preset }
func (t NetworkPolicyTemplateSource) networkPolicyPresetMixed() bool {
	return len(t.Ingress) > 0 || len(t.Egress) > 0 || len(t.PolicyTypes) > 0
}

// CollectNetworkPolicyPresetEntries returns all preset references declared for
// this CRD's networkPolicies across OnCreate, OnReconcile, and OnDelete.
// Only entries with a non-empty Preset string are returned.
func (c *CRDEntry) CollectNetworkPolicyPresetEntries() []NetworkPolicyPresetEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []NetworkPolicyPresetEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			pp, ok := res.(networkPolicyPresetd)
			if !ok {
				return
			}
			preset := pp.getNetworkPolicyPreset()
			if preset == "" {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			out = append(out, NetworkPolicyPresetEntry{
				Phase:        phase,
				ResourceName: rname,
				Preset:       preset,
				Mixed:        pp.networkPolicyPresetMixed(),
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
