package coordinator

import (
	"context"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/metrics"
	"github.com/intentd/inrun/pkg/runtime/coordinator/reconcile"
	"github.com/intentd/inrun/pkg/runtime/queue"
)

// Worker that only processes items for a specific GVK
func (c *Controller) runWorkerForGVK(ctx context.Context, gvk string, workerID string) {
	wq, ok := c.queueRegistry.For(gvk)

	if !ok {
		logger.Warn().Str("gvk", gvk).Msg("no queue for CRD. Using default queue")
		wq = c.defaultWorkqueue
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
			c.crdHealthMap[gvk].MarkWorkerProcessing(workerID)

			// Process the item
			func() {
				defer wq.Done(item)
				c.processItemForGVK(ctx, gvk, item)
			}()

			// Back to idle after processing
			c.crdHealthMap[gvk].MarkWorkerIdle(workerID)

			// Update metrics (outside of processing state)
			depth := float64(wq.Depth())
			metrics.SetQueueDepth(gvk, depth)

			// Push live depth into AutoMetrics so the autoscaler can read it.
			// No-op for non-autoscaled CRDs (runtimeMap entry has no autoscaler).
			c.mu.RLock()
			rt := c.runtimeMap[gvk]
			c.mu.RUnlock()
			if rt != nil {
				rt.autoMetrics.SetQueueDepth(int64(wq.Depth()))
			}

			// Resource count — read from this CRD's informer cache
			if entry, ok := c.catalog.Get(gvk); ok && entry.Informer != nil {
				count := float64(len(entry.Informer.GetIndexer().List()))
				metrics.SetResourceCount(gvk, count)
			}
		}
	}
}

// processItemForGVK takes one item off the queue: it handles the queue's own
// cases (drain, wrong GVK, no reconciler), runs one pass, and schedules the
// next from its Outcome.
func (c *Controller) processItemForGVK(ctx context.Context, gvk string, item queue.QueueItem) {
	if err := ctx.Err(); err != nil {
		logger.Debug().Msg("process item: context cancelled")
		return
	}

	// Resolve queue — per-CRD if registered, default otherwise
	wq, ok := c.queueRegistry.For(gvk)
	if !ok {
		wq = c.defaultWorkqueue
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
	c.mu.RLock()
	rec := c.reconcilers[gvk]
	c.mu.RUnlock()

	if rec == nil {
		logger.Error().Str("gvk", gvk).Str("key", item.Key).Msg("no reconciler found — dropping item")
		wq.Forget(item)
		return
	}

	entry, ok := c.catalog.Get(gvk)
	if !ok {
		logger.Error().Str("gvk", gvk).Str("key", item.Key).Msg("no catalog entry — dropping item")
		wq.Forget(item)
		return
	}

	out := reconcile.Run(ctx, reconcile.Deps{
		Kube:     c.kube,
		Event:    c.event,
		Cat:      c.cat,
		Registry: c.catalog,
	}, reconcile.Input{
		GVK:              gvk,
		Key:              item.Key,
		Entry:            entry,
		Reconciler:       rec,
		Health:           c.crdHealthMap[gvk],
		FailureThreshold: c.failureThreshold[gvk],
		Sentinels:        wq.Sentinels(item),
	})

	if out.Err != nil {
		wq.AddRateLimited(item)
	} else {
		wq.Forget(item)
	}
	if out.Next >= 0 {
		wq.AddAfter(item, out.Next)
	}
}
