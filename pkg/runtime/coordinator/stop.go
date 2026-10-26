package coordinator

import (
	"context"
	"time"

	"github.com/intentd/inrun/pkg/logger"
)

// stopCRDWorkers cancels the CRD context and waits for all workers to drain.
func (dc *DependencyCoordinator) stopCRDWorkers(ctx context.Context, gvk string) {
	dc.mu.RLock()
	cancel, okCancel := dc.cancelFuncs[gvk]
	wg, okWG := dc.wgs[gvk]
	dc.mu.RUnlock()

	// Step 1: signal workers to stop accepting new work
	if okCancel {
		cancel()
	}

	// Step 2: shut down the queue — this unblocks any worker blocked on
	// queue.GetWithContext() waiting for the next item.
	if wq, ok := dc.queueReg.For(gvk); ok {
		wq.Shutdown(ctx)
	}

	if !okWG {
		return
	}

	// Step 3: reset worker counts after shutdown
	if health, ok := dc.crdHealthMap[gvk]; ok {
		health.ResetWorkerCounts()
		health.MarkWorkersStopped()
	}

	// Step 4: wait for workers to drain — with a timeout.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info().Str("gvk", gvk).Msg("workers drained cleanly")
	case <-time.After(dc.drainTimeout):
		logger.Warn().Str("gvk", gvk).
			Dur("timeout", dc.drainTimeout).
			Msg("drain timeout exceeded — workers may still be running. " +
				"Consider increasing SHUTDOWN_TIMEOUT if reconciles call slow external APIs.")
	}
}
