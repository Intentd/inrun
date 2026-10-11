package reconcile

import (
	"context"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/template"
)

// resolveRequeueAfter returns the post-reconcile requeue delay.
// 0 means re-enqueue immediately; -1 means wait for an informer event.
func resolveRequeueAfter(
	ctx context.Context,
	d Deps,
	in Input,
	result domain.Result,
	prepared *domain.PreparedRequest,
) time.Duration {
	key := in.Key
	if after, reason := result.EffectiveRequeue(); after > 0 {
		if result.Requeue != nil && result.RequeueAfter > 0 {
			logger.FromContext(ctx).
				Warn().Str("key", key).
				Msg("result sets both Requeue and RequeueAfter; using Requeue")
		}
		if reason != "" {
			logger.FromContext(ctx).
				Info().Str("key", key).
				Str("after", after.String()).Str("reason", reason).Msg("requeue requested")
		}
		return after
	}
	obj := objectFromCache(in.Entry, key)
	var resolver *template.Resolver
	if prepared != nil {
		resolver = prepared.Context.(*template.Resolver)
	} else if obj != nil {
		if r, err := template.NewResolver(ctx, obj); err == nil {
			resolver = r.WithUserNotes(d.Cat.UserNotes()).
				WithProfiles(d.Cat.UserProfiles()).
				WithHealth(in.Health.HealthAsMap()).
				WithMetrics(in.Health.GetAutoMetrics())
		}
	}
	if after := d.Cat.EvaluateRequeue(ctx, in.Entry.CRD.Name, obj, resolver); after > 0 {
		return after
	}
	if prepared != nil {
		if box := prepare.BoxFrom(prepared); box.HasCleanup() {
			return 0
		}
	}
	return -1
}
