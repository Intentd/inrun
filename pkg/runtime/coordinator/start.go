package coordinator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/autoscaler"
	"github.com/intentd/inrun/pkg/types"
)

// startCRDWorkers activates a CRD: builds runtime state, wires health
// callbacks, builds and registers the reconciler, then starts all goroutines.
func (dc *DependencyCoordinator) startCRDWorkers(ctx context.Context, gvk string, workers int) {
	entry, ok := dc.catalog.Get(gvk)
	if !ok {
		logger.Fatal().Str("gvk", gvk).Msg("no catalog entry found")
		return
	}

	crdCtx, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	rt := dc.buildCRDRuntime(crdCtx, gvk, workers, wg, entry.CRD)
	dc.mu.Lock()
	dc.runtimeMap[gvk] = rt
	dc.mu.Unlock()

	dc.wireCRDHealthCallbacks(gvk, rt, workers, entry.CRD)

	rec := entry.ReconcilerFactory()

	if rt.autoscaler != nil {
		go rt.autoscaler.Run(crdCtx)
	}
	if entry.CRD.OperatorBox.EffectiveAutoscale() != nil {
		dc.startResyncLoop(crdCtx, gvk)
	}

	dc.mu.Lock()
	dc.reconcilers[gvk] = rec
	dc.cancelFuncs[gvk] = cancel
	dc.wgs[gvk] = wg
	dc.crdHealthMap[gvk].SetStarted()
	dc.crdHealthMap[gvk].SetTotalWorkers(int32(workers))
	dc.crdHealthMap[gvk].SetGVK(gvk)
	dc.started[gvk] = true
	dc.mu.Unlock()

	dc.launchWorkers(crdCtx, gvk, workers, wg)
	dc.observer.Observe(crdCtx, entry.CRD)
}

// buildCRDRuntime creates the per-CRD semaphore, AutoMetrics, spawnWorker
// closure, and (when declared) the autoscaler. The returned runtime is not yet
// registered in runtimeMap — the caller does that under the lock.
func (dc *DependencyCoordinator) buildCRDRuntime(
	crdCtx context.Context,
	gvk string,
	workers int,
	wg *sync.WaitGroup,
	crd types.CRDEntry,
) *perCRDRuntime {
	sem := autoscaler.NewResizableSemaphore(workers)
	autoMet := autoscaler.NewAutoMetrics(sem)
	rt := &perCRDRuntime{
		sem:         sem,
		autoMetrics: autoMet,
	}

	workerCounter := atomic.Int64{}
	rt.spawnWorker = func() {
		n := workerCounter.Add(1)
		wg.Add(1)
		workerID := workerID(gvk, "autoscale-worker", int(n))
		dc.crdHealthMap[gvk].MarkStartupWorkerIdle(workerID)
		go func(id string) {
			defer wg.Done()
			dc.runWorkerForGVK(crdCtx, gvk, id)
		}(workerID)
	}

	wq, _ := dc.queueReg.For(gvk)
	if crd.AutoscaleEnabled() {
		target := &coordinatorTarget{rt: rt, wq: wq, gvk: gvk}
		baseline := types.AutoscaleBaseline{
			Workers:  workers,
			MaxDepth: crd.SetQueueDepth(0),
			Resync:   crd.SetResync(0),
		}
		rt.autoscaler = autoscaler.NewAutoscaler(
			dc.kube.Clientset(),
			crd.APITypes.Kind,
			crd.Box().EffectiveAutoscale(),
			baseline,
			target,
			autoMet,
			crd.Box().EffectiveCross(),
		)
	}

	autoscaler.GlobalCrossMetricsRegistry.Register(crd.Name, autoMet)
	return rt
}

// wireCRDHealthCallbacks registers AutoMetrics and WorkerInfo providers into
// the CRD's health tracker so the /catalog endpoint can read them.
func (dc *DependencyCoordinator) wireCRDHealthCallbacks(
	gvk string,
	rt *perCRDRuntime,
	workers int,
	crd types.CRDEntry,
) {
	dc.crdHealthMap[gvk].SetAutoMetricsFn(rt.autoMetrics.AsMap)

	dc.crdHealthMap[gvk].SetWorkerInfoFn(func() *autoscaler.WorkerInfo {
		maxWorkers := workers
		if rt.autoscaler != nil {
			if snap := rt.autoscaler.Snapshot(); snap != nil && snap.EffectiveWorkers > maxWorkers {
				maxWorkers = snap.EffectiveWorkers
			}
		}
		info := autoscaler.BuildWorkerInfo(
			rt.sem,
			rt.autoMetrics,
			workers,
			crd.SetQueueDepth(0),
			crd.SetResync(0).String(),
			maxWorkers,
			rt.autoscaler != nil,
			rt.autoscaler.Snapshot(),
		)
		return &info
	})
}

// launchWorkers spawns the baseline worker goroutines for a CRD.
func (dc *DependencyCoordinator) launchWorkers(ctx context.Context, gvk string, workers int, wg *sync.WaitGroup) {
	for i := 0; i < workers; i++ {
		wg.Add(1)
		id := workerID(gvk, "worker", i)
		dc.crdHealthMap[gvk].MarkStartupWorkerIdle(id)
		go func(workerID string) {
			defer wg.Done()
			dc.runWorkerForGVK(ctx, gvk, workerID)
		}(id)
	}
}

// workerID builds a worker identifier from a GVK, role, and index.
func workerID(gvk, role string, n int) string {
	id := fmt.Sprintf("%s-%s-%d", gvk, role, n)
	id = strings.ReplaceAll(id, ",", "")
	id = strings.ReplaceAll(id, " ", "-")
	return id
}
