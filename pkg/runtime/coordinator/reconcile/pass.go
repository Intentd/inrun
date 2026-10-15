package reconcile

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/event"
	"github.com/intentd/inrun/pkg/intent"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/runtime/coordinator/apply"
	"github.com/intentd/inrun/pkg/runtime/coordinator/contract"
	"github.com/intentd/inrun/pkg/runtime/coordinator/maintain"
	"github.com/intentd/inrun/pkg/runtime/coordinator/prepare"
	"github.com/intentd/inrun/pkg/runtime/coordinator/vitals"
	"github.com/intentd/inrun/pkg/template"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// Deps are the long-lived dependencies a pass uses, held by the coordinator.
type Deps struct {
	Kube     kubeclient.Interface
	Event    event.Recorder
	Cat      *catalog.Catalog
	Registry prepare.CatalogRegistry
}

// Input is one pass: a key of a CRD, its reconciler and its health.
type Input struct {
	GVK              string
	Key              string
	Entry            contract.RegistryEntry
	Reconciler       domain.Reconciler
	Health           *vitals.CRDHealth
	FailureThreshold int
	// Sentinels are the queue item's sentinel values, for the reconcile gate.
	Sentinels map[string]string
}

// Outcome tells the worker what to do with the key after a pass.
type Outcome struct {
	// Err is set when the pass failed: retry with backoff.
	Err error
	// Next is when to run the key again; negative waits for the next event.
	Next time.Duration
	// Skipped is set when nothing ran: the key was gated or its object is gone.
	Skipped bool
}

// Run runs one pass for in.Key.
func Run(ctx context.Context, d Deps, in Input) Outcome {
	gvk, key, entry := in.GVK, in.Key, in.Entry

	// Pre-reconcile gate: evaluate operatorBox.preReconcile.when/or conditions.
	// The reconciler is never called when conditions are not met — gated state
	// is idle, not failure; error rate and health state are unaffected.
	if entry.CRD.HasAnyReconcileGate() {
		// A CR being deleted is never gated: its finalizing reconcile releases
		// Inrun's finalizer.
		if obj := objectFromCache(entry, key); obj == nil || obj.GetDeletionTimestamp() == nil {
			if gated, reason := evaluatePreReconcileCheck(ctx, d, obj, entry.CRD.Name, in.Sentinels); gated {
				in.Health.RecordGated(reason)
				return Outcome{Next: -1, Skipped: true}
			}
		}
	}

	// Prepare: build the enriched request (normalize, resolver, cross, validation…).
	// Returns nil when the object is not in cache (deleted between dequeue and here)
	// or when the namespace guard blocks it — both are silent skips, not errors.
	prepared, valResult, err := prepare.Prepare(ctx, prepare.Input{
		Entry:    entry,
		Key:      key,
		Cat:      d.Cat,
		Kube:     d.Kube,
		Registry: d.Registry,
	})
	if err != nil {
		logger.Error().Err(err).Str("gvk", gvk).Str("key", key).Msg("prepare failed")
		return Outcome{Err: err, Next: -1}
	}
	if prepared == nil {
		return Outcome{Next: -1, Skipped: true}
	}

	// Box and resolver are extracted once here and shared by cleanup and maintain.
	// Cleanup never suspends reconciling: a due cleanup deletes the CR here; a
	// pending one only shapes when the next pass runs.
	box, resolver := prepare.BoxFrom(prepared), prepared.Context.(*template.Resolver)
	check, err := checkCleanup(ctx, d.Kube, prepared.Object, box, resolver)
	if err != nil {
		logger.Error().Err(err).Str("gvk", gvk).Str("key", key).Msg("cleanup evaluation failed")
		return Outcome{Err: err, Next: -1}
	}
	if err := maintain.Apply(ctx, maintain.Input{
		CRD:        entry.CRD,
		Cat:        d.Cat,
		Kube:       d.Kube,
		Recorder:   d.Event,
		ForCleanup: check.due,
	}, prepared.Object, box, resolver); err != nil {
		logger.Error().Err(err).Str("gvk", gvk).Str("key", key).Msg("maintain failed")
		return Outcome{Err: err, Next: -1}
	}
	if check.due {
		if err := deleteCR(ctx, d, prepared.Object, check.reason); err != nil {
			logger.Error().Err(err).Str("gvk", gvk).Str("key", key).Msg("cleanup: delete failed")
			return Outcome{Err: err, Next: -1}
		}
		return Outcome{Next: -1}
	}
	cleanup := check.cleanupWait

	// safeReconcile catches panics
	result, reconcileErr := safeReconcile(ctx, in, prepared)

	// A CR being deleted: Inrun's cleanup finalizer holds it until neither the
	// Result nor a conditional reconcile.requeue asks to wait, or until
	// runtime.finalizers.timeout passes. Only that requeue counts; the Result's
	// writes are ignored so nothing recreates children of a CR on its way out.
	finalizing := prepared.Finalizing
	if finalizing {
		after, reason := result.EffectiveRequeue()
		if after <= 0 && reconcileErr == nil {
			after, reason = declarativeRequeue(ctx, d, in, prepared), "reconcile.requeue"
		}
		done := reconcileErr == nil && after <= 0
		expired, left := finalizeDeadline(prepared.Object, box.EffectiveFinalizerConfig(), time.Now())
		ns, name := prepared.Object.GetNamespace(), prepared.Object.GetName()

		if done || expired {
			if !done {
				why := reason
				if reconcileErr != nil {
					why = reconcileErr.Error()
				}
				d.Event.Eventf(prepared.Object, corev1.EventTypeWarning, "FinalizeTimeout",
					"Still finalizing after runtime.finalizers.timeout; removing Inrun's finalizer anyway (last: %s)", why)
				reconcileErr = nil
			}
			// Children owner references cannot reach go before the finalizer.
			// After a timeout the CR is released even if this fails.
			if _, err := deleteChildren(ctx, d.Kube, entry.CRD, prepared); err != nil {
				if done {
					reconcileErr = fmt.Errorf("deleting children: %w", err)
				} else {
					logger.Warn().Err(err).Str("key", key).Msg("finalize timeout: deleting children failed")
				}
			}
			// On any error the finalizer stays and the CR is retried below.
			if reconcileErr == nil {
				if err := maintain.Finalize(ctx, d.Kube, prepared.Object, box); err != nil {
					reconcileErr = fmt.Errorf("removing finalizers: %w", err)
				} else {
					if done {
						d.Event.Eventf(prepared.Object, corev1.EventTypeNormal, "Finalized",
							"Reconciler finished; removed Inrun's finalizer from %s/%s", ns, name)
					}
					return Outcome{Next: -1}
				}
			}
		} else if reconcileErr == nil {
			// Never sleep past the timeout.
			if left > 0 && after > left {
				after = left
			}
			result = domain.Result{Requeue: &domain.Requeue{After: after, Reason: reason}}
		}
	}

	// Apply the resources, labels and annotations the Result returns. Every
	// reconciler's writes take this one path; a failure is a reconcile failure.
	// Children it applied before and no longer returns are pruned when the
	// effective reconcile.prune is enabled.
	if reconcileErr == nil && !finalizing {
		prune := entry.CRD.EffectivePrune(intent.Target(prepared.Object.GetAnnotations()))
		pruned, err := apply.Result(ctx, d.Kube, prepared.Object, entry.CRD.AllManagedResources(), prune, result)
		for _, p := range pruned {
			d.Event.Eventf(prepared.Object, corev1.EventTypeNormal, "Pruned",
				"Deleted %s %s: no longer in the result", p.Kind, path.Join(p.Namespace, p.Name))
		}
		if err != nil {
			d.Event.Eventf(prepared.Object, corev1.EventTypeWarning, "ApplyFailed",
				"Applying the result for %s/%s: %v", prepared.Object.GetNamespace(), prepared.Object.GetName(), err)
			reconcileErr = err
		}
	}

	// A CR that moved to another serve target loses what the previous one
	// created, now that the new target's children are in place.
	if reconcileErr == nil && !finalizing {
		if err := sweepPreviousSurface(ctx, d.Kube, entry.CRD, prepared.Object); err != nil {
			reconcileErr = fmt.Errorf("surface sweep: %w", err)
		}
	}

	// Post: status patch + emit — always runs, even on reconcile failure.
	reconcileErr = runPost(ctx, postInput{
		Kube:      d.Kube,
		Event:     d.Event,
		Health:    in.Health,
		CRD:       entry.CRD,
		Prepared:  prepared,
		Result:    result,
		ValResult: toPostValResult(valResult),
	}, reconcileErr)

	if reconcileErr != nil {
		d.Event.Eventf(prepared.Object, corev1.EventTypeWarning, "ReconcileError",
			"Reconciling %s: %v", key, reconcileErr)
		logger.Error().Err(reconcileErr).Str("gvk", gvk).Str("key", key).Msg("reconcile failed")
		// A failing reconcile never delays a pending cleanup.
		next := time.Duration(-1)
		if cleanup.pending {
			next = cleanup.wait
		}
		return Outcome{Err: reconcileErr, Next: next}
	}

	d.Event.Eventf(prepared.Object, corev1.EventTypeNormal, "Reconciled", "Reconciled %s", key)

	// Record or withdraw the cleanup this Result requests. A failed reconcile
	// (above) changes neither, and a CR being deleted has nothing to clean up.
	if !finalizing {
		w, err := settleResultCleanup(ctx, d.Kube, prepared.Object, result.Cleanup)
		if err != nil {
			logger.Warn().Err(err).Str("gvk", gvk).Str("key", key).Msg("cleanup: could not record the result's request")
		}
		if w.pending {
			cleanup.add(w.wait)
		}
	}

	// Requeue and cleanup are independent: wake at whichever is due first; the
	// next pass recomputes both.
	next := resolveRequeueAfter(ctx, d, in, result, prepared)
	if cleanup.pending && (next < 0 || cleanup.wait < next) {
		next = cleanup.wait
	}
	return Outcome{Next: next}
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
func safeReconcile(ctx context.Context, in Input, prepared *domain.PreparedRequest) (result domain.Result, err error) {
	gvk, key := in.GVK, in.Key

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
			in.Health.RecordFailure(err, in.FailureThreshold)
			metrics.RecordReconcile(gvk, "error")
		}
	}()

	// Inject per-request fields into the logr context so ctrl.LoggerFrom(ctx)
	// automatically carries the resource key on every reconciler log line.
	ctx = ctrllog.IntoContext(ctx, ctrllog.FromContext(ctx).WithValues("resource", key, "gvk", gvk))

	// Execute the operator's reconcile logic.
	// Any returned error is treated as a reconcile failure.
	ns, name, _ := cache.SplitMetaNamespaceKey(key)

	prepare.RenderArgs(prepared)
	result, err = in.Reconciler.Reconcile(ctx, domain.Request{
		Key:            key,
		NamespacedName: apitypes.NamespacedName{Namespace: ns, Name: name},
		Prepared:       prepared,
	})
	if err != nil {
		// Update CRD health state and metrics.
		in.Health.RecordFailure(err, in.FailureThreshold)
		metrics.RecordReconcile(gvk, "error")
		return result, err
	}

	// Successful reconcile path.
	in.Health.RecordSuccess()
	vitals.InvalidateChildrenCache(ns, name)
	metrics.RecordReconcile(gvk, "success")
	return result, nil
}
