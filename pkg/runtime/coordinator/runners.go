package coordinator

import (
	"context"
	"time"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/contract"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/maintain"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/prepare"
	"github.com/inrundev/inrun/pkg/template"
	"github.com/inrundev/inrun/pkg/types"
)

// runMaintain applies labels, annotations, and finalizers before reconcile.
// ForCleanup=true strips deletion-protection so the subsequent Delete call
// bypasses the webhook selector.
func (k *Controller) runMaintain(
	ctx context.Context,
	entry contract.RegistryEntry,
	obj domain.Object,
	box types.OperatorBoxConfig,
	resolver *template.Resolver,
	forCleanup bool,
) error {
	return maintain.Apply(ctx, maintain.Input{
		CRD:        entry.CRD,
		Cat:        k.cat,
		Kube:       k.kube,
		Recorder:   k.event,
		ForCleanup: forCleanup,
	}, obj, box, resolver)
}

// resolveRequeueAfter returns the post-reconcile requeue delay.
// 0 means re-enqueue immediately; -1 means wait for an informer event.
func (k *Controller) resolveRequeueAfter(
	ctx context.Context,
	gvk string,
	entry contract.RegistryEntry,
	key string,
	result domain.Result,
	prepared *domain.PreparedRequest,
) time.Duration {
	if d, reason := result.EffectiveRequeue(); d > 0 {
		if result.Requeue != nil && result.RequeueAfter > 0 {
			logger.FromContext(ctx).
				Warn().Str("key", key).
				Msg("result sets both Requeue and RequeueAfter; using Requeue")
		}
		if reason != "" {
			logger.FromContext(ctx).
				Info().Str("key", key).
				Str("after", d.String()).Str("reason", reason).Msg("requeue requested")
		}
		return d
	}
	obj := k.objectFromCache(entry, key)
	var resolver *template.Resolver
	if prepared != nil {
		resolver = prepared.Context.(*template.Resolver)
	} else if obj != nil {
		if r, err := template.NewResolver(ctx, obj); err == nil {
			health := k.crdHealthMap[gvk]
			resolver = r.WithUserNotes(k.cat.UserNotes()).
				WithProfiles(k.cat.UserProfiles()).
				WithHealth(health.HealthAsMap()).
				WithMetrics(health.GetAutoMetrics())
		}
	}
	if d := k.cat.EvaluateRequeue(ctx, entry.CRD.Name, obj, resolver); d > 0 {
		return d
	}
	if prepared != nil {
		if box := prepare.BoxFrom(prepared); box.HasCleanup() {
			return 0
		}
	}
	return -1
}
