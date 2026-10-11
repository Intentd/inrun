package coordinator

import (
	"context"
	"fmt"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cleanup has two independent sources, each with durable state on the CR so the
// clock survives restarts and leader changes:
//
//   - the Catalog's operatorBox.runtime.cleanup conditions, evaluated before
//     reconcile; cleanupPendingAnnotation records when they were first met;
//   - the reconciler's Result.Cleanup, known only after reconcile;
//     cleanupDueAnnotation records when it falls due.
//
// Cleanup never suspends reconciling. Each pass deletes the CR when any cleanup
// is due; otherwise it reconciles and schedules the next pass at the earlier of
// the requeue and the nearest pending cleanup.
const (
	cleanupPendingAnnotation = "inrun.dev/cleanup-pending-since"
	cleanupDueAnnotation     = "inrun.dev/cleanup-due"
	cleanupReasonAnnotation  = "inrun.dev/cleanup-reason"
)

const (
	catalogCleanupReason    = "catalog cleanup conditions met"
	reconcilerCleanupReason = "requested by the reconciler"
)

// cleanupWait is the time until the nearest pending cleanup is due.
type cleanupWait struct {
	pending bool
	wait    time.Duration
}

// add folds one pending cleanup into w, keeping the nearest.
func (w *cleanupWait) add(remaining time.Duration) {
	if remaining < 0 {
		remaining = 0
	}
	if !w.pending || remaining < w.wait {
		w.pending, w.wait = true, remaining
	}
}

// cleanupCheck is the pre-reconcile cleanup state.
type cleanupCheck struct {
	due    bool   // delete now
	reason string // why, for the event and the log
	cleanupWait
}

// consider records one cleanup source that is remaining away from its due time.
func (c *cleanupCheck) consider(remaining time.Duration, reason string) {
	if remaining <= 0 {
		if !c.due {
			c.due, c.reason = true, reason
		}
		return
	}
	c.add(remaining)
}

// annotationPatcher is the part of the kube client cleanup needs.
type annotationPatcher interface {
	Patch(ctx context.Context, obj domain.Object, patch kubeclient.Patch, opts metav1.PatchOptions) error
}

// checkCleanup settles the Catalog's cleanup conditions and reports whether any
// cleanup, from the Catalog or from an earlier Result, is due.
func checkCleanup(
	ctx context.Context,
	kube annotationPatcher,
	obj domain.Object,
	box types.OperatorBoxConfig,
	resolver *template.Resolver,
) (cleanupCheck, error) {
	var c cleanupCheck
	ann := obj.GetAnnotations()
	key := obj.GetNamespace() + "/" + obj.GetName()

	if box.HasCleanup() {
		cleanup := box.EffectiveCleanup()
		met := types.EvaluateConditions(resolver.Data(), cleanup.When, cleanup.Or, resolver.TemplateEvaluator())
		since, pending := ann[cleanupPendingAnnotation]

		switch {
		case met && cleanup.After.Duration == 0:
			c.consider(0, catalogCleanupReason)

		case met && !pending:
			now := time.Now().UTC().Format(time.RFC3339)
			if err := patchAnnotations(ctx, kube, obj, map[string]string{cleanupPendingAnnotation: now}); err != nil {
				return c, fmt.Errorf("cleanup: stamping pending annotation: %w", err)
			}
			logger.Info().
				Str("key", key).
				Str("after", cleanup.After.Duration.String()).
				Msg("cleanup: conditions met — grace period started")
			c.consider(cleanup.After.Duration, catalogCleanupReason)

		case met:
			t, err := time.Parse(time.RFC3339, since)
			if err != nil {
				// An unparseable timestamp is treated as elapsed.
				c.consider(0, catalogCleanupReason)
			} else {
				c.consider(cleanup.After.Duration-time.Since(t), catalogCleanupReason)
			}

		case pending:
			// Conditions no longer met: reset the clock.
			if err := patchAnnotations(ctx, kube, obj, nil, cleanupPendingAnnotation); err != nil {
				logger.Warn().Err(err).Str("key", key).Msg("cleanup: failed to clear pending annotation")
			}
		}
	}

	if due, ok := ann[cleanupDueAnnotation]; ok {
		reason := ann[cleanupReasonAnnotation]
		if reason == "" {
			reason = reconcilerCleanupReason
		}
		t, err := time.Parse(time.RFC3339, due)
		if err != nil {
			c.consider(0, reason)
		} else {
			c.consider(time.Until(t), reason)
		}
	}

	return c, nil
}

// settleResultCleanup records or withdraws the cleanup a successful Result
// requests, and returns how long until it is due. The first request starts the
// clock; later requests leave the due time alone. A nil request withdraws it.
func settleResultCleanup(ctx context.Context, kube annotationPatcher, obj domain.Object, req *domain.Cleanup) (cleanupWait, error) {
	var w cleanupWait
	ann := obj.GetAnnotations()
	due, has := ann[cleanupDueAnnotation]
	key := obj.GetNamespace() + "/" + obj.GetName()

	if req == nil {
		if has {
			if err := patchAnnotations(ctx, kube, obj, nil, cleanupDueAnnotation, cleanupReasonAnnotation); err != nil {
				return w, fmt.Errorf("cleanup: withdrawing reconciler request: %w", err)
			}
			logger.Info().Str("key", key).Msg("cleanup: reconciler withdrew its request")
		}
		return w, nil
	}

	if has {
		t, err := time.Parse(time.RFC3339, due)
		if err != nil {
			w.add(0)
		} else {
			w.add(time.Until(t))
		}
		return w, nil
	}

	set := map[string]string{
		cleanupDueAnnotation: time.Now().Add(req.After).UTC().Format(time.RFC3339),
	}
	var remove []string
	if req.Reason != "" {
		set[cleanupReasonAnnotation] = req.Reason
	} else if _, ok := ann[cleanupReasonAnnotation]; ok {
		remove = append(remove, cleanupReasonAnnotation)
	}
	if err := patchAnnotations(ctx, kube, obj, set, remove...); err != nil {
		return w, fmt.Errorf("cleanup: recording reconciler request: %w", err)
	}
	logger.Info().
		Str("key", key).
		Str("after", req.After.String()).
		Str("reason", req.Reason).
		Msg("cleanup: requested by the reconciler")
	w.add(req.After)
	return w, nil
}

// patchAnnotations sets and removes annotations with a JSON merge patch.
// obj itself is never modified: it may be the informer cache's copy.
func patchAnnotations(ctx context.Context, kube annotationPatcher, obj domain.Object, set map[string]string, remove ...string) error {
	base := obj.DeepCopyObject().(domain.Object)
	modified := obj.DeepCopyObject().(domain.Object)

	ann := make(map[string]string, len(modified.GetAnnotations())+len(set))
	for k, v := range modified.GetAnnotations() {
		ann[k] = v
	}
	for k, v := range set {
		ann[k] = v
	}
	for _, k := range remove {
		delete(ann, k)
	}
	modified.SetAnnotations(ann)

	return kube.Patch(ctx, modified, kubeclient.MergeFrom(base), metav1.PatchOptions{})
}

// deleteCR issues a foreground delete for obj. Called by the worker after
// maintain has removed deletion-protection labels (ForCleanup=true path).
// No-ops when the CR already has a deletionTimestamp (deletion in progress).
func (k *Controller) deleteCR(ctx context.Context, obj domain.Object, reason string) error {
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil
	}

	name := obj.GetName()
	ns := obj.GetNamespace()

	propagation := metav1.DeletePropagationForeground
	if err := k.kube.Delete(ctx, obj, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		return fmt.Errorf("cleanup: deleting CR %s/%s: %w", ns, name, err)
	}
	k.event.Eventf(obj, corev1.EventTypeNormal, "CleanedUp", "Deleted %s/%s: %s", ns, name, reason)
	logger.Info().
		Str("key", ns+"/"+name).
		Str("reason", reason).
		Msg("cleanup: CR deleted")
	return nil
}
