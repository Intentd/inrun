package prepare

import (
	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/utils"
)

// RenderArgs renders the request's reconcile.args onto prepared.Args, for the
// reconciler (req.Args()), and adds them to the prepared resolver as .args,
// for everything rendered after them.
func RenderArgs(prepared *domain.PreparedRequest) {
	if prepared == nil {
		return
	}
	prepared.Args = Args(prepared)
	if resolver, ok := prepared.Context.(*template.Resolver); ok && resolver != nil {
		prepared.Context = resolver.WithArgs(prepared.Args)
	}
}

// Args renders the effective operatorBox.reconcile.args (per-target overrides
// already merged) against the CR for this reconcile. Values without template
// expressions pass through unchanged. Returns nil when no args are declared.
func Args(prepared *domain.PreparedRequest) domain.Args {
	if prepared == nil {
		return nil
	}
	resolver, _ := prepared.Context.(*template.Resolver)
	return renderArgs(BoxFrom(prepared), resolver)
}

// renderArgs renders box's reconcile.args against resolver; nil when none are
// declared, unrendered without a resolver.
func renderArgs(box types.OperatorBoxConfig, resolver *template.Resolver) domain.Args {
	if box.Reconcile == nil || len(box.Reconcile.Args) == 0 {
		return nil
	}
	if resolver == nil {
		return domain.Args(box.Reconcile.Args)
	}
	return domain.Args(utils.ResolveArgsMap(box.Reconcile.Args, resolver.TemplateEvaluator()))
}
