package types

// EnvFromRefEntry describes a single envFrom secretRef/configMapRef entry
// found in a resource template. Used by blueprint validation to catch a
// suffix-without-keys config error early at load time.
type EnvFromRefEntry struct {
	Phase        string // "onCreate", "onReconcile", "onDelete"
	ResourceName string // template name field (may be a template expression)
	RefKind      string // "secretRef" or "configMapRef"
	Ref          EnvFromRef
}

// envFromCarrier is implemented by any resource template that carries an envFrom field.
type envFromCarrier interface {
	GetEnvFrom() *EnvFrom
}

func (t DeploymentTemplateSource) GetEnvFrom() *EnvFrom  { return t.EnvFrom }
func (t ReplicaSetTemplateSource) GetEnvFrom() *EnvFrom  { return t.EnvFrom }
func (t StatefulSetTemplateSource) GetEnvFrom() *EnvFrom { return t.EnvFrom }

// CollectEnvFromEntries returns every envFrom secretRef/configMapRef entry
// declared across OnCreate, OnReconcile, and OnDelete for this CRD.
func (c *CRDEntry) CollectEnvFromEntries() []EnvFromRefEntry {
	if !c.HasAnyHookTemplates() {
		return nil
	}

	var out []EnvFromRefEntry

	collect := func(phase string, ht *HookTemplates) {
		if ht == nil {
			return
		}
		ht.VisitResources(func(res interface{}) {
			e, ok := res.(envFromCarrier)
			if !ok {
				return
			}
			ef := e.GetEnvFrom()
			if ef == nil {
				return
			}

			var rname string
			if n, ok := res.(namer); ok {
				rname = n.GetName()
			}

			for _, ref := range ef.SecretRef {
				out = append(out, EnvFromRefEntry{Phase: phase, ResourceName: rname, RefKind: "secretRef", Ref: ref})
			}
			for _, ref := range ef.ConfigMapRef {
				out = append(out, EnvFromRefEntry{Phase: phase, ResourceName: rname, RefKind: "configMapRef", Ref: ref})
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
