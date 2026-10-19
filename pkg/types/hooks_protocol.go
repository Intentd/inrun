package types

// PortProtocolEntry describes a single port protocol declaration found in a
// resource template. Used by blueprint validation to catch invalid protocol
// values early at load time.
type PortProtocolEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // template name field (may be a template expression)
	Protocol     string // raw protocol string as written in the blueprint
}

// ported is implemented by any resource template that carries a port protocol field.
type ported interface {
	GetProtocol() string
}

func (t DeploymentTemplateSource) GetProtocol() string  { return t.Protocol }
func (t ReplicaSetTemplateSource) GetProtocol() string  { return t.Protocol }
func (t StatefulSetTemplateSource) GetProtocol() string { return t.Protocol }
func (t PodTemplateSource) GetProtocol() string         { return t.Protocol }

// CollectPortProtocolEntries returns all non-empty protocol declarations across
// OnCreate, OnReconcile, and OnDelete for this CRD.
func (c *CRDEntry) CollectPortProtocolEntries() []PortProtocolEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []PortProtocolEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			p, ok := res.(ported)
			if !ok {
				return
			}
			proto := p.GetProtocol()
			if proto == "" {
				return // empty means "use TCP default" — nothing to validate
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			out = append(out, PortProtocolEntry{
				Phase:        phase,
				ResourceName: rname,
				Protocol:     proto,
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
