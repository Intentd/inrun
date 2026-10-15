package reconcile

import (
	"fmt"
	"sort"

	"github.com/intentd/inrun/pkg/types"
)

// validateReconcileConditions checks reconcile.when/or, the CRD's and each
// serve target's. They decide whether the reconcile happens, so they are
// evaluated before reconcile.external makes its calls: they may not read
// .external, nor an arg whose template reads it.
func (c *checker) validateReconcileConditions() error {
	notes := c.Cat.UserNotes()
	for name, crd := range c.Cat.EnabledCRDs() {
		if err := checkReconcileConditions(fmt.Sprintf("CRD %q", name), crd.Box().Reconcile, notes); err != nil {
			return err
		}
		if crd.Serve == nil {
			continue
		}
		for target := range crd.Serve.Target.Entries {
			where := fmt.Sprintf("CRD %q serve target %q", name, target)
			if err := checkReconcileConditions(where, crd.EffectiveOperatorBox(target).Reconcile, notes); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkReconcileConditions(where string, rc *types.ReconcileConfig, notes types.NoteRegistry) error {
	if !rc.HasConditions() {
		return nil
	}
	fromCalls := argsReadingExternal(rc.Args, notes)
	for _, cond := range append(append([]types.Condition{}, rc.When...), rc.Or...) {
		for _, expr := range cond.Expressions() {
			if notes.References(expr, "external") {
				return fmt.Errorf("%s %s: reconcile.when/or reads .external (%q). They decide whether the "+
					"reconcile happens, so they are evaluated before reconcile.external makes its calls. "+
					"To decide on a call's result, use preReconcile.reconcileGate",
					failureMark(), where, expr)
			}
			for _, arg := range fromCalls {
				if notes.References(expr, "args."+arg) {
					return fmt.Errorf("%s %s: reconcile.when/or reads args.%s (%q), which reads .external. "+
						"The conditions are evaluated before reconcile.external makes its calls; read the "+
						"CR field the arg maps instead, or decide in preReconcile.reconcileGate",
						failureMark(), where, arg, expr)
				}
			}
		}
	}
	return nil
}

// argsReadingExternal returns the names of the args whose templates read
// .external, sorted.
func argsReadingExternal(args map[string]interface{}, notes types.NoteRegistry) []string {
	var out []string
	for name, v := range args {
		if valueReads(v, "external", notes) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// valueReads reports whether any string in v (a string, map or list) reads root.
func valueReads(v interface{}, root string, notes types.NoteRegistry) bool {
	switch t := v.(type) {
	case string:
		return notes.References(t, root)
	case map[string]interface{}:
		for _, item := range t {
			if valueReads(item, root, notes) {
				return true
			}
		}
	case []interface{}:
		for _, item := range t {
			if valueReads(item, root, notes) {
				return true
			}
		}
	}
	return false
}
