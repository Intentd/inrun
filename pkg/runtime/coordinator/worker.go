package coordinator

import (
	"context"
	"fmt"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/apply"
	corev1 "k8s.io/api/core/v1"
	"runtime"
	"time"

	"github.com/inrundev/inrun/domain"
	"github.com/inrundev/inrun/pkg/logger"
	"github.com/inrundev/inrun/pkg/metrics"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/post"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/prepare"
	"github.com/inrundev/inrun/pkg/runtime/coordinator/vitals"
	"github.com/inrundev/inrun/pkg/runtime/queue"
	"github.com/inrundev/inrun/pkg/template"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// Worker that only processes items for a specific GVK
func (k *Controller) runWorkerForGVK(ctx context.Context, gvk string, workerID string) {
	wq, ok := k.queueRegistry.For(gvk)

	if !ok {
		logger.Warn().Str("gvk", gvk).Msg("no queue for CRD. Using default queue")
		wq = k.defaultWorkqueue
	}

	logger.Debug().Str("worker_id", workerID).Str("gvk", gvk).Msg("worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Debug().Str("worker_id", workerID).Str("gvk", gvk).Msg("worker stopped")
			return
		default:
			// Worker is idle - waiting for work
			// (already idle from previous iteration or initialization)

			// Wait for an item
			item, shutdown := wq.Get()
			if shutdown {
				logger.Debug().Str("worker_id", workerID).Str("gvk", gvk).Msg("worker stopping (queue shutdown)")
				return
			}

			// Worker is now processing
			k.crdHealthMap[gvk].MarkWorkerProcessing(workerID)

			// Process the item
			func() {
				defer wq.Done(item)
				k.processItemForGVK(ctx, gvk, item)
			}()

			// Back to idle after processing
			k.crdHealthMap[gvk].MarkWorkerIdle(workerID)

			// Update metrics (outside of processing state)
			depth := float64(wq.Depth())
			metrics.SetQueueDepth(gvk, depth)

			// Push live depth into AutoMetrics so the autoscaler can read it.
			// No-op for non-autoscaled CRDs (runtimeMap entry has no autoscaler).
			k.mu.RLock()
			rt := k.runtimeMap[gvk]
			k.mu.RUnlock()
			if rt != nil {
				rt.autoMetrics.SetQueueDepth(int64(wq.Depth()))
			}

			// Resource count — read from this CRD's informer cache
			if entry, ok := k.catalog.Get(gvk); ok && entry.Informer != nil {
				count := float64(len(entry.Informer.GetIndexer().List()))
				metrics.SetResourceCount(gvk, count)
			}
		}
	}
}

// processItemForGVK handles a single reconciliation item
func (k *Controller) processItemForGVK(ctx context.Context, gvk string, item queue.QueueItem) {
	if err := ctx.Err(); err != nil {
		logger.Debug().Msg("process item: context cancelled")
		return
	}

	// Resolve queue — per-CRD if registered, default otherwise
	wq, ok := k.queueRegistry.For(gvk)
	if !ok {
		wq = k.defaultWorkqueue
	}

	// Added to help shutdown workers and preserve the queue on missing crds
	// After dequeuing, they check ctx.Done() at the top of the loop and exit.
	// The queue is intact for reactivation. No ShutDown() called.
	// TODO: Currently does not shutdown the workers
	if item.Key == drainSentinel {
		wq.Forget(item)
		return
	}

	// With per-CRD queues this check is only needed for the default queue path
	// where multiple GVKs share one queue
	if item.GVK != gvk {
		if ok {
			// This item is in the wrong per-CRD queue — should not happen
			// Log and drop rather than spin
			logger.Error().
				Str("expected", gvk).
				Str("got", item.GVK).
				Msg("GVK mismatch in per-CRD queue — dropping item")
			wq.Forget(item)
		} else {
			// Default queue — item belongs to a different GVK, put it back
			// This is the only valid re-queue case
			wq.AddRateLimited(item)
		}
		return
	}

	// Look up the pre-built reconciler
	k.mu.RLock()
	rec := k.reconcilers[gvk]
	k.mu.RUnlock()

	if rec == nil {
		logger.Error().Str("gvk", gvk).Str("key", item.Key).Msg("no reconciler found — dropping item")
		wq.Forget(item)
		return
	}

	// Pre-reconcile gate: evaluate operatorBox.preReconcile.when/or conditions.
	// The reconciler is never called when conditions are not met — gated state
	// is idle, not failure; error rate and health state are unaffected.
	entry, hasEntry := k.catalog.Get(gvk)
	if hasEntry && entry.CRD.HasAnyReconcileGate() {
		obj := k.objectFromCache(entry, item.Key)
		sentinelMap := wq.Sentinels(item)
		if gated, reason := k.evaluatePreReconcileCheck(ctx, obj, entry.CRD.Name, sentinelMap); gated {
			k.crdHealthMap[gvk].RecordGated(reason)
			wq.Forget(item)
			return
		}
	}

	// Prepare: build the enriched request (normalize, resolver, cross, validation…).
	// Returns nil when the object is not in cache (deleted between dequeue and here)
	// or when the namespace guard blocks it — both are silent skips, not errors.
	var prepared *domain.PreparedRequest
	var valResult *prepare.ValidationResult
	if hasEntry {
		var prepErr error
		prepared, valResult, prepErr = prepare.Prepare(ctx, prepare.Input{
			Entry:    entry,
			Key:      item.Key,
			Cat:      k.cat,
			Kube:     k.kube,
			Registry: k.catalog,
		})
		if prepErr != nil {
			logger.Error().Err(prepErr).Str("gvk", gvk).Str("key", item.Key).Msg("prepare failed")
			wq.AddRateLimited(item)
			k.failedReconcile(gvk)
			return
		}
		if prepared == nil {
			wq.Forget(item)
			return
		}
	}

	// Box and resolver are extracted once here and shared by cleanup and maintain.
	// Cleanup never suspends reconciling: a due cleanup deletes the CR here; a
	// pending one only shapes when the next pass runs.
	var cleanup cleanupWait
	if hasEntry && prepared != nil {
		box, resolver := prepare.BoxFrom(prepared), prepared.Context.(*template.Resolver)

		check, cleanupErr := checkCleanup(ctx, k.kube, prepared.Object, box, resolver)
		if cleanupErr != nil {
			logger.Error().Err(cleanupErr).Str("gvk", gvk).Str("key", item.Key).Msg("cleanup evaluation failed")
			wq.AddRateLimited(item)
			k.failedReconcile(gvk)
			return
		}

		if maintErr := k.runMaintain(ctx, entry, prepared.Object, box, resolver, check.due); maintErr != nil {
			logger.Error().Err(maintErr).Str("gvk", gvk).Str("key", item.Key).Msg("maintain failed")
			wq.AddRateLimited(item)
			k.failedReconcile(gvk)
			return
		}

		if check.due {
			if delErr := k.deleteCR(ctx, prepared.Object, check.reason); delErr != nil {
				logger.Error().Err(delErr).Str("gvk", gvk).Str("key", item.Key).Msg("cleanup: delete failed")
				wq.AddRateLimited(item)
				k.failedReconcile(gvk)
				return
			}
			wq.Forget(item)
			k.successReconcile(gvk)
			return
		}
		cleanup = check.cleanupWait
	}

	// safeReconcile catches panics
	result, reconcileErr := k.safeReconcile(rec, k.crdHealthMap[gvk], ctx, item.Key, gvk, prepared)

	// Apply the resources, labels and annotations the Result returns. Every
	// reconciler's writes take this one path; a failure is a reconcile failure.
	if reconcileErr == nil && hasEntry && prepared != nil {
		if err := apply.Result(ctx, k.kube, prepared.Object, entry.CRD.AllManagedResources(), result); err != nil {
			k.event.Eventf(prepared.Object, corev1.EventTypeWarning, "ApplyFailed",
				"Applying the result for %s/%s: %v", prepared.Object.GetNamespace(), prepared.Object.GetName(), err)
			reconcileErr = err
		}
	}

	// Post: status patch + emit — always runs, even on reconcile failure.
	if hasEntry && prepared != nil {
		reconcileErr = runPost(ctx, PostInput{
			Kube:      k.kube,
			Event:     k.event,
			Health:    k.crdHealthMap[gvk],
			CRD:       entry.CRD,
			Prepared:  prepared,
			Result:    result,
			ValResult: toPostValResult(valResult),
		}, reconcileErr)
	}

	if reconcileErr != nil {
		logger.Error().Err(reconcileErr).Str("gvk", gvk).Str("key", item.Key).Msg("reconcile failed")
		wq.AddRateLimited(item)
		// A failing reconcile never delays a pending cleanup.
		if cleanup.pending {
			wq.AddAfter(item, cleanup.wait)
		}
		k.failedReconcile(gvk)
		return
	}

	// Record or withdraw the cleanup this Result requests. A failed reconcile
	// (above) changes neither.
	if hasEntry && prepared != nil {
		w, err := settleResultCleanup(ctx, k.kube, prepared.Object, result.Cleanup)
		if err != nil {
			logger.Warn().Err(err).Str("gvk", gvk).Str("key", item.Key).Msg("cleanup: could not record the result's request")
		}
		if w.pending {
			cleanup.add(w.wait)
		}
	}

	// Requeue and cleanup are independent: wake at whichever is due first; the
	// next pass recomputes both.
	wq.Forget(item)
	next := k.resolveRequeueAfter(ctx, gvk, entry, item.Key, result, prepared)
	if cleanup.pending && (next < 0 || cleanup.wait < next) {
		next = cleanup.wait
	}
	if next >= 0 {
		wq.AddAfter(item, next)
	}
}

// safeReconcile wraps a Reconciler's Reconcile() call in a fully isolated,
// panic‑protected execution boundary. This is the core of Inrun’s
// "operator sandbox" model: each CRD runs in its own safe compartment,
// ensuring that failures in one operator never cascade into others.
//
// Responsibilities:
//   - Measure reconcile duration for metrics
//   - Catch and convert panics into errors (preventing controller crash)
//   - Record success/failure into CRDHealth
//   - Emit success/error metrics
//   - Apply degrade thresholds for health tracking
//
// This function guarantees that no matter what happens inside rec.Reconcile(),
// the controller process stays alive and the failure is reported deterministically.
func (k *Controller) safeReconcile(
	rec domain.Reconciler,
	health *vitals.CRDHealth,
	ctx context.Context,
	key string,
	gvk string,
	prepared *domain.PreparedRequest,
) (result domain.Result, err error) {

	// Track how long this reconcile took.
	// The defer ensures duration is recorded even if a panic occurs.
	start := time.Now()
	defer func() {
		metrics.ObserveReconcileDuration(gvk, time.Since(start).Seconds())

		// Panic recovery: this is the isolation boundary.
		// Any panic inside the operator is caught, logged, and converted into an error.
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)

			err = fmt.Errorf("reconciler panic: %v", r)

			logger.Error().
				Str("gvk", gvk).
				Str("key", key).
				Str("panic", fmt.Sprint(r)).
				Str("stack", string(buf[:n])).
				Msg("reconciler panic recovered")

			// Update CRD health state and metrics.
			health.RecordFailure(err, k.failureThreshold[gvk])
			metrics.RecordReconcile(gvk, "error")

			// TODO: track panic stats differently with recovery
		}
	}()

	// Inject per-request fields into the logr context so ctrl.LoggerFrom(ctx)
	// automatically carries the resource key on every reconciler log line.
	ctx = ctrllog.IntoContext(ctx, ctrllog.FromContext(ctx).WithValues("resource", key, "gvk", gvk))

	// Execute the operator's reconcile logic.
	// Any returned error is treated as a reconcile failure.
	ns, name, _ := cache.SplitMetaNamespaceKey(key)

	prepared.Args = prepare.Args(prepared)
	result, err = rec.Reconcile(ctx, domain.Request{
		Key:            key,
		NamespacedName: apitypes.NamespacedName{Namespace: ns, Name: name},
		Prepared:       prepared,
	})
	if err != nil {
		// Update CRD health state and metrics.
		health.RecordFailure(err, k.failureThreshold[gvk])
		metrics.RecordReconcile(gvk, "error")
		return result, err
	}

	// Successful reconcile path.
	health.RecordSuccess()
	k.successReconcile(gvk)
	vitals.InvalidateChildrenCache(ns, name)
	metrics.RecordReconcile(gvk, "success")
	return result, nil
}

// toPostValResult converts a prepare.ValidationResult to the post package's
// equivalent type for status condition writing.
func toPostValResult(v *prepare.ValidationResult) *post.ValidationResult {
	if v == nil {
		return nil
	}
	r := &post.ValidationResult{Deny: v.Deny}
	for _, viol := range v.Violations {
		r.Violations = append(r.Violations, post.ValidationViolation{
			Field:   viol.Field,
			Rule:    viol.Rule,
			Value:   viol.Value,
			Message: viol.Message,
		})
	}
	for _, w := range v.Warnings {
		r.Warnings = append(r.Warnings, post.ValidationViolation{
			Field:   w.Field,
			Rule:    w.Rule,
			Value:   w.Value,
			Message: w.Message,
		})
	}
	return r
}
