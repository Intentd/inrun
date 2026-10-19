package reconcile

import (
	"fmt"
)

// validateRequeue checks that requeue.after is either a template expression
// or a valid duration string accepted by core.ParseTimeDuration (KEL).
// Empty string is allowed — it means no requeue.
func (c *checker) validateRequeue() error {
	for crdName, crd := range c.Blueprint.EnabledCRDs() {
		rc := crd.Box().Reconcile
		if rc == nil || rc.Requeue == nil {
			continue
		}
		if err := checkRequeueAfter(fmt.Sprintf("crd %q: requeue.after", crdName), rc.Requeue.After); err != nil {
			return err
		}
	}
	return nil
}

// checkRequeueAfter checks one requeue.after value: empty (no requeue), a
// template, or a duration core.ParseTimeDuration (KEL) accepts. where names it in
// the error.
func checkRequeueAfter(where, after string) error {
	if after == "" || isTemplate(after) {
		return nil
	}
	if _, err := parseTimeDuration(after); err != nil {
		return fmt.Errorf("%s %s %q is not a valid duration or template expression. "+
			"Use a Go duration string (e.g. \"30s\", \"5m\") or a template (e.g. '{{ .spec.interval | default \"60s\" }}')",
			failureMark(), where, after)
	}
	return nil
}
