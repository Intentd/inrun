package types

import (
	"fmt"

	"github.com/intentd/inrun/domain"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ── Registrations ─────────────────────────────────────────────────────────────
// The compiled Go a CRD runs, looked up in the registries `inrun generate
// registry` fills. Nothing is copied onto the entry: the runtime calls these
// when it builds reconcilers and informers, and Catalog validation calls the
// same methods to report a missing registration before anything starts.

// NeedsConstructor reports whether this CRD runs a compiled constructor: a
// typed CRD that is neither reconciled by default nor remote.
func (c *CRDEntry) NeedsConstructor() bool {
	return !c.IsDynamic() && !c.DefaultReconcile() && !c.WithRemoteDecl()
}

// Constructor returns the compiled constructor registered for this CRD's GVK.
func (c *CRDEntry) Constructor() (NewReconcilerFunc, bool) {
	fn, ok := ReconcilerRegistry[c.GroupVersionKind]
	return fn, ok && fn != nil
}

// HookFactory returns the compiled hook factory for a CRD reconciled by
// default; false when none is registered or the CRD runs its own reconciler.
func (c *CRDEntry) HookFactory() (func() domain.AnyReconcileHooks, bool) {
	if !c.DefaultReconcile() {
		return nil, false
	}
	fn, ok := HookRegistry[c.GroupVersionKind]
	return fn, ok && fn != nil
}

// CheckSharedTargetHooks reports a serve target that declares hooks in the
// CRD-level binary (or only overrides their args) while no CRD-level hook
// factory is registered. Targets with a distinct binary are checked by
// TargetHookFactories.
func (c *CRDEntry) CheckSharedTargetHooks() error {
	if !c.DefaultReconcile() || !c.HasServeTargetEntries() {
		return nil
	}
	if _, ok := c.HookFactory(); ok {
		return nil
	}
	crdLevel := c.crdLevelHookLocation()
	for target, cfg := range c.Serve.Target.Entries {
		if cfg.OperatorBox == nil || cfg.Box().Reconcile == nil {
			continue
		}
		h := cfg.Box().Reconcile.Hooks
		if h == nil || (h.Location != "" && h.Location != crdLevel) {
			continue
		}
		if h.Location != "" || h.Function != "" {
			return fmt.Errorf(
				"target %q: reconciler.hooks declared but no hook factory registered for GVK %s — "+
					"re-run inrun generate registry",
				target, c.GroupVersionKind,
			)
		}
	}
	return nil
}

// TargetHookFactories returns the hook factories of serve targets that declare
// a hook binary distinct from the CRD-level one, keyed by target. Targets that
// share the CRD-level binary and only override args are absent: they run the
// CRD-level hooks with their own args.
func (c *CRDEntry) TargetHookFactories() (map[string]func() domain.AnyReconcileHooks, error) {
	if !c.HasServeTargetEntries() {
		return nil, nil
	}
	crdLevel := c.crdLevelHookLocation()
	var out map[string]func() domain.AnyReconcileHooks
	for target, cfg := range c.Serve.Target.Entries {
		if cfg.OperatorBox == nil || cfg.Box().Reconcile == nil {
			continue
		}
		h := cfg.Box().Reconcile.Hooks
		if h == nil || h.Location == "" || h.Location == crdLevel {
			continue
		}
		byTarget, ok := TargetHookRegistry[c.GroupVersionKind]
		if !ok {
			return nil, fmt.Errorf(
				"target %q: per-target hooks (location %q) declared but "+
					"no TargetHookRegistry entry for this GVK — re-run inrun generate registry",
				target, h.Location,
			)
		}
		fn, ok := byTarget[target]
		if !ok {
			return nil, fmt.Errorf(
				"target %q: no TargetHookRegistry entry for this target — re-run inrun generate registry",
				target,
			)
		}
		if out == nil {
			out = make(map[string]func() domain.AnyReconcileHooks)
		}
		out[target] = fn
	}
	return out, nil
}

// TargetConstructors returns the constructors of serve targets that declare
// reconcile.default: false with a constructor, keyed by target.
func (c *CRDEntry) TargetConstructors() (map[string]NewReconcilerFunc, error) {
	if !c.HasServeTargetEntries() {
		return nil, nil
	}
	var out map[string]NewReconcilerFunc
	for target, cfg := range c.Serve.Target.Entries {
		if cfg.OperatorBox == nil || cfg.Box().Reconcile == nil {
			continue
		}
		rec := cfg.Box().Reconcile
		if rec.Default == nil || *rec.Default || rec.ConstructorDecl == nil {
			continue
		}
		byTarget, ok := TargetReconcilerRegistry[c.GroupVersionKind]
		if !ok {
			return nil, fmt.Errorf(
				"target %q: reconciler.default: false declared but "+
					"no TargetReconcilerRegistry entry for this GVK — re-run inrun generate registry",
				target,
			)
		}
		fn, ok := byTarget[target]
		if !ok {
			return nil, fmt.Errorf(
				"target %q: no TargetReconcilerRegistry entry for this target — re-run inrun generate registry",
				target,
			)
		}
		if out == nil {
			out = make(map[string]NewReconcilerFunc)
		}
		out[target] = fn
	}
	return out, nil
}

// RuntimeObjects returns a new object and list for this CRD: unstructured for
// a dynamic CRD, the registered Go types for a typed one.
func (c *CRDEntry) RuntimeObjects() (runtime.Object, runtime.Object, error) {
	gvk := c.GroupVersionKind
	if c.IsDynamic() {
		g, v, k := c.APITypes.Group, c.APITypes.Version, c.APITypes.Kind
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: g, Version: v, Kind: k})
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: g, Version: v, Kind: k + "List"})
		return obj, list, nil
	}
	objFn, ok := ObjectRegistry[gvk]
	if !ok {
		return nil, nil, fmt.Errorf("no object type registered for %s — re-run inrun generate registry", gvk)
	}
	listFn, ok := ListRegistry[gvk]
	if !ok {
		return nil, nil, fmt.Errorf("no list type registered for %s — re-run inrun generate registry", gvk)
	}
	return objFn(), listFn(), nil
}

// crdLevelHookLocation is the location of the CRD-level hooks binary, or "".
func (c *CRDEntry) crdLevelHookLocation() string {
	if r := c.Box().Reconcile; r != nil && r.Hooks != nil {
		return r.Hooks.Location
	}
	return ""
}
