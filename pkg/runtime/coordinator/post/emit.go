package post

import (
	"context"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
)

// applyEmit fires declarative Kubernetes events declared in operatorBox.emit.events.
// Each entry is evaluated in the order Go iterates the map (non-deterministic);
// when order matters, names should include a sequence prefix in the catalog.
// Condition failures and template resolution errors are logged and skipped — they
// never fail or requeue the reconcile.
func applyEmit(
	ctx context.Context,
	in Input,
	obj domain.Object,
	resolver *template.Resolver,
	box types.OperatorBoxConfig,
	reconcileErr error,
) {
	if in.Recorder == nil || !box.HasEmit() {
		return
	}

	log := logger.FromContext(ctx)
	data := resolver.Data()

	for name, entry := range box.EmitEntries() {
		if entry == nil {
			continue
		}

		if !entry.EmitOn(reconcileErr) {
			log.Debug().
				Str("crd", in.CRD.GVKString()).
				Str("name", obj.GetName()).
				Str("event", name).
				Msg("emit: on trigger not matched — skipped")
			continue
		}

		if !types.EvaluateConditions(data, entry.When, entry.Or, resolver.TemplateEvaluator()) {
			log.Debug().
				Str("crd", in.CRD.GVKString()).
				Str("name", obj.GetName()).
				Str("event", name).
				Msg("emit: conditions not met — skipped")
			continue
		}

		reason, err := resolver.Resolve(entry.Reason)
		if err != nil {
			log.Warn().Err(err).
				Str("crd", in.CRD.GVKString()).
				Str("name", obj.GetName()).
				Str("event", name).
				Msg("emit: failed to resolve reason — skipped")
			continue
		}

		message, err := resolver.Resolve(entry.Message)
		if err != nil {
			log.Warn().Err(err).
				Str("crd", in.CRD.GVKString()).
				Str("name", obj.GetName()).
				Str("event", name).
				Msg("emit: failed to resolve message — skipped")
			continue
		}

		in.Recorder.Eventf(obj, string(entry.Type), reason, "%s", message)

		log.Debug().
			Str("crd", in.CRD.GVKString()).
			Str("name", obj.GetName()).
			Str("event", name).
			Str("reason", reason).
			Msg("emit: event fired")
	}
}
