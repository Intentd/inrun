package coordinator

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/autoscaler"
	"github.com/intentd/inrun/pkg/runtime/queue"
)

// perCRDRuntime holds the per-CRD concurrency and autoscale state owned by
// the Controller. Previously embedded in generic.Reconciler.
type perCRDRuntime struct {
	sem         *autoscaler.ResizableSemaphore
	autoMetrics *autoscaler.AutoMetrics
	autoscaler  *autoscaler.Autoscaler
	resyncNs    atomic.Int64
	spawnWorker func()
}

// coordinatorTarget implements autoscaler.AutoscaleTarget backed by the
// Controller's owned state for a specific GVK. Created by startCRDWorkers.
type coordinatorTarget struct {
	rt  *perCRDRuntime
	wq  *queue.Workqueue
	gvk string
}

func (t *coordinatorTarget) ResizeWorkers(n int) {
	old := t.rt.sem.Capacity()
	t.rt.sem.Resize(n)
	if n > old && t.rt.spawnWorker != nil {
		for i := old; i < n; i++ {
			go t.rt.spawnWorker()
		}
	}
	logger.Info().
		Str("crd", t.gvk).
		Int("workers", n).
		Msg("autoscaler: worker pool resized")
}

func (t *coordinatorTarget) SetQueueDepthLimit(n int) {
	if t.wq != nil {
		t.wq.SetQueueDepth(n)
	}
	logger.Info().
		Str("crd", t.gvk).
		Int("queueDepth", n).
		Msg("autoscaler: queue depth limit updated")
}

func (t *coordinatorTarget) SetResyncInterval(d time.Duration) {
	t.rt.resyncNs.Store(d.Nanoseconds())
	logger.Info().
		Str("crd", t.gvk).
		Dur("resync", d).
		Msg("autoscaler: resync interval updated")
}

// startResyncLoop runs the adjustable resync goroutine for a CRD.
// Idle (polling 500ms) when resyncNs == 0; fires at the stored interval otherwise.
// Mirrors the resync logic that was previously on generic.Reconciler.
func (c *Controller) startResyncLoop(ctx context.Context, gvk string) {
	rt := c.runtimeMap[gvk]
	entry, ok := c.catalog.Get(gvk)
	if !ok || entry.Informer == nil {
		return
	}
	wq, _ := c.queueRegistry.For(gvk)
	go func() {
		for {
			ns := rt.resyncNs.Load()
			if ns == 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(500 * time.Millisecond):
				}
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(ns)):
				if wq == nil {
					continue
				}
				items := entry.Informer.GetIndexer().List()
				for _, obj := range items {
					wq.Enqueue(obj, gvk)
				}
				logger.Debug().
					Str("crd", gvk).
					Int("count", len(items)).
					Dur("interval", time.Duration(ns)).
					Msg("autoscaler: resync re-enqueued all objects")
			}
		}
	}()
}
