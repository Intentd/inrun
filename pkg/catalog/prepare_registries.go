package catalog

import (
	"fmt"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/types"
)

// ---------------------------------------------------------------------------------
// Add consstructors
func (k *Catalog) AddConstructors() error {
	for name, crd := range k.enabledCRDs {
		if !crd.IsDynamic() {
			if crd.DefaultReconcile() {
				// Per-target constructors (reconcile.default: false on a target)
				// are wired by AddTargetConstructors from TargetReconcilerRegistry.
				continue
			}

			// Remote reconciler: no compiled constructor — dispatched by coordinator at runtime.
			if crd.WithRemoteDecl() {
				k.enabledCRDs[name] = crd
				continue
			}

			constructorFn, ok := types.ReconcilerRegistry[crd.GroupVersionKind]
			if !ok {
				return fmt.Errorf(
					"CRD %q: no constructor registered — "+
						"check reconciler.constructor in Catalog and re-run inrun generate registry",
					name,
				)
			}

			if crd.Box().Reconcile == nil {
				crd.Box().Reconcile = &types.ReconcileConfig{}
			}
			crd.Box().Reconcile.Constructor = constructorFn
		}

		k.enabledCRDs[name] = crd
	}
	return nil
}

// ---------------------------------------------------------------------------------
// Add hooks
func (k *Catalog) AddHooks() error {
	for name, crd := range k.enabledCRDs {
		if !crd.DefaultReconcile() {
			continue
		}
		hookFn, ok := types.HookRegistry[crd.GroupVersionKind]
		if ok {
			if crd.Box().Reconcile == nil {
				crd.Box().Reconcile = &types.ReconcileConfig{}
			}
			crd.Box().Reconcile.HookFactory = hookFn
		}

		if !crd.HasServeTargetEntries() {
			k.enabledCRDs[name] = crd
			continue
		}

		// Per-target hook validation: a target that declares hooks at the same
		// location as the CRD-level binary (or just overrides args) relies on the
		// CRD-level factory — it must be registered. Targets with a distinct binary
		// are validated separately by addTargetHooks.
		crdLevelHookLoc := ""
		if crd.Box().Reconcile != nil && crd.Box().Reconcile.Hooks != nil {
			crdLevelHookLoc = crd.Box().Reconcile.Hooks.Location
		}
		for targetName, targetCfg := range crd.Serve.Target.Entries {
			if targetCfg.OperatorBox == nil || targetCfg.Box().Reconcile == nil {
				continue
			}
			h := targetCfg.Box().Reconcile.Hooks
			if h == nil {
				continue
			}
			// Distinct binary → addTargetHooks validates; skip here.
			if h.Location != "" && h.Location != crdLevelHookLoc {
				continue
			}
			// Same binary (or args-only override) — CRD-level factory must be registered.
			if (h.Location != "" || h.Function != "") && !ok {
				return fmt.Errorf(
					"CRD %q target %q: reconciler.hooks declared but no hook factory registered for GVK %s — "+
						"re-run inrun generate registry",
					name, targetName, crd.GroupVersionKind,
				)
			}
		}

		k.enabledCRDs[name] = crd
	}
	return nil
}

// ---------------------------------------------------------------------------------
// AddTargetHooks wires per-target hook factories from TargetHookRegistry onto
// CRDEntry.TargetHookFactories. Only targets that declare a distinct hook binary
// (different location from the CRD-level hooks) need an entry here — targets that
// share the CRD-level binary and only override args are handled at reconcile time
// by mergeReconcilerConfig inside EffectiveOperatorBox.
func (k *Catalog) AddTargetHooks() error {
	for name, crd := range k.enabledCRDs {
		if !crd.HasServeTargetEntries() {
			continue
		}
		crdLevelLocation := ""
		if crd.Box().Reconcile != nil && crd.Box().Reconcile.Hooks != nil {
			crdLevelLocation = crd.Box().Reconcile.Hooks.Location
		}
		gvk := crd.GroupVersionKind
		for targetName, targetCfg := range crd.Serve.Target.Entries {
			if targetCfg.OperatorBox == nil || targetCfg.Box().Reconcile == nil {
				continue
			}
			h := targetCfg.Box().Reconcile.Hooks
			if h == nil || h.Location == "" || h.Location == crdLevelLocation {
				continue
			}
			targetMap, ok := types.TargetHookRegistry[gvk]
			if !ok {
				return fmt.Errorf(
					"CRD %q target %q: per-target hooks (location %q) declared but "+
						"no TargetHookRegistry entry for this GVK — re-run inrun generate registry",
					name, targetName, h.Location,
				)
			}
			fn, ok := targetMap[targetName]
			if !ok {
				return fmt.Errorf(
					"CRD %q target %q: no TargetHookRegistry entry for this target — "+
						"re-run inrun generate registry",
					name, targetName,
				)
			}
			if crd.TargetHookFactories == nil {
				crd.TargetHookFactories = make(map[string]func() domain.AnyReconcileHooks)
			}
			crd.TargetHookFactories[targetName] = fn
		}
		k.enabledCRDs[name] = crd
	}
	return nil
}

// AddTargetConstructors wires per-target constructor factories from
// TargetReconcilerRegistry onto CRDEntry.TargetReconcilerFactories.
func (k *Catalog) AddTargetConstructors() error {
	for name, crd := range k.enabledCRDs {
		if !crd.HasServeTargetEntries() {
			continue
		}
		gvk := crd.GroupVersionKind
		for targetName, targetCfg := range crd.Serve.Target.Entries {
			if targetCfg.OperatorBox == nil || targetCfg.Box().Reconcile == nil {
				continue
			}
			rec := targetCfg.Box().Reconcile
			if rec.Default == nil || *rec.Default || rec.ConstructorDecl == nil {
				continue
			}
			targetMap, ok := types.TargetReconcilerRegistry[gvk]
			if !ok {
				return fmt.Errorf(
					"CRD %q target %q: reconciler.default: false declared but "+
						"no TargetReconcilerRegistry entry for this GVK — re-run inrun generate registry",
					name, targetName,
				)
			}
			fn, ok := targetMap[targetName]
			if !ok {
				return fmt.Errorf(
					"CRD %q target %q: no TargetReconcilerRegistry entry for this target — "+
						"re-run inrun generate registry",
					name, targetName,
				)
			}
			if crd.TargetReconcilerFactories == nil {
				crd.TargetReconcilerFactories = make(map[string]types.NewReconcilerFunc)
			}
			crd.TargetReconcilerFactories[targetName] = fn
		}
		k.enabledCRDs[name] = crd
	}
	return nil
}
