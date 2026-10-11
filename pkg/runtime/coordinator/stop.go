package coordinator

import (
	"context"
	"time"

	"github.com/intentd/inrun/pkg/logger"
)

// stopCRDWorkers cancels the CRD context and waits for all workers to drain.
func (k *DependencyCoordinator) stopCRDWorkers(ctx context.Context, gvk string) {
	k.mu.RLock()
	cancel, okCancel := k.cancelFuncs[gvk]
	wg, okWG := k.wgs[gvk]
	k.mu.RUnlock()

	// Step 1: signal workers to stop accepting new work
	if okCancel {
		cancel()
	}

	// Step 2: shut down the queue — this unblocks any worker blocked on
	// queue.GetWithContext() waiting for the next item.
	if wq, ok := k.queueReg.For(gvk); ok {
		wq.Shutdown(ctx)
	}

	if !okWG {
		return
	}

	// Step 3: reset worker counts after shutdown
	if health, ok := k.crdHealthMap[gvk]; ok {
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
	case <-time.After(k.drainTimeout):
		logger.Warn().Str("gvk", gvk).
			Dur("timeout", k.drainTimeout).
			Msg("drain timeout exceeded — workers may still be running. " +
				"Consider increasing SHUTDOWN_TIMEOUT if reconciles call slow external APIs.")
	}
}
