package prepare

import (
	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/utils"
)

// Args renders the effective operatorBox.reconcile.args (per-target overrides
// already merged) against the CR for this reconcile. Values without template
// expressions pass through unchanged. Returns nil when no args are declared.
func Args(prepared *domain.PreparedRequest) domain.Args {
	if prepared == nil {
		return nil
	}
	box := BoxFrom(prepared)
	if box.Reconcile == nil || len(box.Reconcile.Args) == 0 {
		return nil
	}
	resolver, ok := prepared.Context.(*template.Resolver)
	if !ok || resolver == nil {
		return domain.Args(box.Reconcile.Args)
	}
	return domain.Args(utils.ResolveArgsMap(box.Reconcile.Args, resolver.TemplateEvaluator()))
}
