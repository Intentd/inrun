package coordinator

import (
	"context"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/util/kube"

	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/runtime/informer"
	"github.com/intentd/inrun/pkg/runtime/queue"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// retryMissingCRDs runs continuously to detect and activate CRDs that were missing at startup
// or deferred because dependencies were not ready.
func (dc *DependencyCoordinator) retryMissingCRDs(ctx context.Context) {
	retryInterval := postStartRetryInterval
	if !kube.IsRunningInPod() {
		retryInterval = postStartRetryIntervalDev
	}
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	backoff := PostStartBackoff
	nameToGVK := dc.NameToGVKMap()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			runtimeMissing := make(map[string]*informer.InformerEntry)

			// ── Custom child CRDs ────────────────────────────────────────────
			// Check GVKs declared in onCreate.custom / onReconcile.custom blocks.
			// These CRDs must exist for the reconciler to create instances via the
			// dynamic client. Failures are silent by default; this makes them visible.
			if len(dc.missingChildGVKs) > 0 {
				stillMissingChild := make(map[string]schema.GroupVersionKind)
				for gvkStr, gvk := range dc.missingChildGVKs {
					gvkCopy := gvk
					ok, err := dc.crdExists(&gvkCopy)
					if err != nil {
						logger.Warn().Str("gvk", gvkStr).Err(err).Msg("crdExists transient error — keeping child in missing list")
						stillMissingChild[gvkStr] = gvk
						continue
					}
					if ok {
						logger.Info().Str("gvk", gvkStr).Msg("custom child CRD now available — refreshing RESTMapper")
						dc.kube.RefreshMapper()
					} else {
						logger.Warn().Str("gvk", gvkStr).Msg("custom child CRD still not available — retrying")
						stillMissingChild[gvkStr] = gvk
					}
				}
				dc.missingChildGVKs = stillMissingChild
			}

			// ───────────────────────────────────────────────
			// 1. Detect CRDs that disappeared at runtime
			// ───────────────────────────────────────────────
			registered := dc.informerFactory.Registered()
			for gvkStr, entry := range registered {
				if entry == nil || entry.Missing {
					continue
				}
				ok, err := dc.crdExists(entry.GVK)
				if err != nil {
					// Transient API server error — cannot determine CRD state.
					// Skip all health changes; we will re-check on the next tick.
					logger.Warn().Str("gvk", gvkStr).Err(err).Msg("crdExists transient error — skipping health update")
					continue
				}
				if !ok {
					logger.Warn().Str("gvk", gvkStr).Msg("CRD disappeared at runtime — marking missing")
					entry.Missing = true
					runtimeMissing[gvkStr] = entry
					dc.informerFactory.SetMissingOnStartup(runtimeMissing)

					// Health tracking
					// 	CRD
					if h := dc.crdHealthMap[gvkStr]; h != nil {
						h.SetMissingAtRuntime()
						h.SetDegraded()
					}

					// 	Catalog
					dc.allOnline.Store(false)
					dc.inrunHealth.SetAllNotOnline()
					dc.inrunHealth.SetCatalogDegraded()

					// Degrade dependents that require healthy condition
					deps := dc.depGraph.GetDependents(entry.Name)
					for _, dep := range deps {
						crd := dc.depGraph.GetNode(dep).CRD
						if crd.DependsOn[entry.Name].Condition == string(types.DependencyConditionHealthy) {
							if h := dc.crdHealthMap[crd.GVKString()]; h != nil {
								h.SetDegraded()
							}
						} else {
							logger.Info().Str("gvk", gvkStr).Msgf("dependency %s is unhealthy", entry.Name)
						}
					}

					if !dc.deactivated[gvkStr] {
						if h := dc.crdHealthMap[gvkStr]; h != nil {
							h.StartedAt()
						}
						logger.Info().Str("gvk", gvkStr).Msg("stopping workers")
						dc.deactivateCRD(gvkStr)
					}
				}
			}

			// ───────────────────────────────────────────────
			// 2. Handle CRDs currently missing
			// ───────────────────────────────────────────────
			missing := dc.informerFactory.Missing()
			if len(missing) == 0 {
				backoff = PostStartBackoff
				logger.Debug().Msg("retry loop: no missing CRDs")
				// Continue to Phase 3 before marking all online
			} else {
				backoff = PostStartBackoff
				logger.Debug().Msgf("retry loop: checking %d missing CRD(s)", len(missing))
				stillMissing := make(map[string]*informer.InformerEntry)

				for gvkStr, entry := range missing {
					ok, err := dc.crdExists(entry.GVK)
					if err != nil {
						// Transient error — cannot determine whether CRD exists.
						// Keep it in the missing list and retry next tick without
						// touching health state (avoids "not started" flip on a
						// pending CRD caused by an API server blip).
						logger.Warn().Str("gvk", gvkStr).Err(err).Msg("crdExists transient error — keeping in missing list")
						stillMissing[gvkStr] = entry
						continue
					}
					if ok {
						dc.activateCRD(ctx, entry)
						dc.deactivated[gvkStr] = false
						if h := dc.crdHealthMap[gvkStr]; h != nil {
							h.SetStarted()
						}
					} else {
						stillMissing[gvkStr] = entry
						if h := dc.crdHealthMap[gvkStr]; h != nil {
							h.SetMissingAtRuntime()
							h.SetDegraded()
						}
						dc.inrunHealth.SetAllNotOnline()
						dc.inrunHealth.SetCatalogDegraded()
						logger.Debug().Msgf("retry loop: %s still not available", gvkStr)
					}
				}
				dc.informerFactory.SetMissingOnStartup(stillMissing)

				if len(stillMissing) > 0 {
					logger.Info().Msgf("retry loop: %d CRD(s) still missing", len(stillMissing))
					dc.allOnline.Store(false)
					dc.inrunHealth.SetAllNotOnline()
					dc.inrunHealth.SetCatalogDegraded()
				}
			}

			// ───────────────────────────────────────────────────────────
			// 3. Activate deferred CRDs (skipped at startup because
			//    dependencies were not ready)
			// ───────────────────────────────────────────────────────────
			for _, name := range dc.depGraph.StartupOrder() {
				gvk := nameToGVK[name]

				dc.mu.RLock()
				started := dc.started[gvk]
				dc.mu.RUnlock()
				if started {
					continue // already running
				}

				// Skip if it's already in the missing map (handled by Phase 2)
				if dc.informerFactory.IsMissing(gvk) {
					continue
				}

				crd := dc.depGraph.GetNode(name).CRD
				if !dc.dependenciesReady(crd, nameToGVK) {
					continue // still waiting for dependencies
				}

				// CRD exists and dependencies are ready → activate
				logger.Info().Str("crd", name).Msg("dependencies now satisfied, activating")
				entry, ok := dc.informerFactory.Registered()[gvk]
				if !ok || entry == nil {
					logger.Error().Str("crd", name).Str("gvk", gvk).Msg("no informer entry found for deferred CRD")
					continue
				}
				dc.activateCRD(ctx, entry)
				dc.deactivated[gvk] = false
				dc.crdHealthMap[gvk].SetStarted()
			}

			// ───────────────────────────────────────────────
			// 4. Update overall health
			// ───────────────────────────────────────────────
			if len(dc.informerFactory.Missing()) == 0 && dc.allCRDsStarted() {
				dc.allOnline.Store(true)
				dc.inrunHealth.SetAllOnline()
				dc.inrunHealth.SetCatalogReady()
				logger.Debug().Msg("retry loop: all CRDs active")
			}

			// Exponential backoff only when CRDs are still missing
			if len(dc.informerFactory.Missing()) > 0 {
				time.Sleep(backoff)
				if backoff < PostStartBackoffMax {
					backoff *= 2
					if backoff > PostStartBackoffMax {
						backoff = PostStartBackoffMax
					}
				}
			}
		}
	}
}

// allCRDsStarted returns true if every CRD in the graph has started.
func (dc *DependencyCoordinator) allCRDsStarted() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	for _, node := range dc.depGraph.GetNodes() {
		gvk := node.CRD.GroupVersionKind.String()
		if !dc.started[gvk] {
			return false
		}
	}
	return true
}

// activateCRD brings a previously missing CRD online after it appears in the cluster.
//
// The informer for this CRD was already created during factory initialization
// but was never started because the CRD did not exist at startup.
// This method starts the existing informer, launches its worker pool, and signals
// any dependents that this CRD is now ready.
//
// This method does NOT recreate the informer — it reuses the one created during
// factory setup, which avoids the "sharedIndexInformer started twice" warning.
//
// Flow:
//  1. Start the existing (but not yet running) informer
//  2. Remove from missing map so it won't be retried again
//  3. Start worker goroutines for this CRD
//  4. Update health tracking to reflect started state
//  5. Close the ready channel to unblock any CRDs that depend on this one
//
// Parameters:
//   - ctx: context for cancellation propagation
//   - entry: the informer entry from the missing map (contains the pre-created informer)
func (dc *DependencyCoordinator) activateCRD(ctx context.Context, entry *informer.InformerEntry) {
	gvkStr := entry.GVK.String()
	name := entry.Name

	logger.Info().Msgf("activating CRD %s (%s)", name, gvkStr)

	// Safety check — the informer should exist because it was created during factory init
	if entry.Informer == nil {
		logger.Error().Msgf("activateCRD: informer for %s is nil", gvkStr)
		return
	}

	// Start the existing informer (it was created but not started at startup)
	// Only start informer if it was never started (startup-missing case).
	if entry.WasNeverStarted { // you can track this with a bool on InformerEntry
		go entry.Informer.Run(ctx.Done())
		entry.WasNeverStarted = false
		logger.Info().Msgf("activateCRD: informer started for %s", name)
	}

	// Mark as no longer missing so the retry loop won't keep processing it
	entry.Missing = false
	dc.informerFactory.RemoveMissing(gvkStr)

	// Start the worker pool for this CRD
	workers := dc.catalog.GetWorkers(gvkStr, dc.defaultWorkers)
	dc.startCRDWorkers(ctx, gvkStr, workers)

	// Update health tracking
	dc.deactivated[gvkStr] = false
	dc.crdHealthMap[gvkStr].SetStarted()
	dc.crdHealthMap[gvkStr].SetTotalWorkers(int32(workers))

	// Signal dependents that this CRD has now started
	// The started channel was created during Coordinate and may still be open
	if ch, exists := dc.startedCh[gvkStr]; exists {
		select {
		case <-ch:
			// Channel already closed (should not happen, but safe)
			logger.Debug().Msgf("activateCRD: started channel for %q was already closed", name)
		default:
			close(ch)
			dc.crdHealthMap[gvkStr].SetStarted()
			deps := dc.depGraph.GetDependents(name)
			if len(deps) > 0 {
				logger.Info().Msgf("activateCRD: closed started channel for %q — unblocking %d dependent(s): %s",
					name, len(deps), strings.Join(deps, ", "))
			}
		}
	}

	// Check dependencies whose dependency condition is 'healthy' and leave healthy channel open
	deps := dc.depGraph.GetDependents(name)
	if len(deps) > 0 {
		for _, dep := range deps {
			crd := dc.NameToCRD(dep)
			if crd.DependsOn.ConditionHealthy(name) {
				if ch, exists := dc.healthyCh[crd.GVKString()]; exists {
					select {
					case <-ch:
						// Channel already closed (should not happen, but safe)
						logger.Debug().Msgf("activateCRD: healthy channel for %q was already closed", name)
					default:
						if !dc.crdHealthMap[crd.GVKString()].IsHealthy() {
							continue
						}
						close(ch)
						dc.crdHealthMap[crd.GVKString()].SetStarted()
						logger.Info().Msgf("activateCRD: closed healthy channel for %q", name)
					}
				}
			}
		}
	}

	logger.Info().Msgf("CRD %s activated", name)
}

// deactivateCRD — drain without permanent shutdown
func (dc *DependencyCoordinator) deactivateCRD(gvk string) {
	dc.mu.RLock()
	cancel, okCancel := dc.cancelFuncs[gvk]
	wg, okWG := dc.wgs[gvk]
	dc.mu.RUnlock()

	if okCancel {
		cancel() // signals workers via context
	}

	// Add a single sentinel item to each worker's queue to unblock
	// any worker currently waiting in GetWithContext.
	// Workers check ctx.Done() after dequeuing — sentinel is dropped.
	if wq, ok := dc.queueReg.For(gvk); ok {
		numWorkers := dc.catalog.GetWorkers(gvk, dc.defaultWorkers)
		for i := 0; i < numWorkers; i++ {
			wq.Add(queue.QueueItem{GVK: gvk, Key: drainSentinel})
		}
	}

	if !okWG {
		return
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		dc.crdHealthMap[gvk].ResetWorkerCounts()
		dc.crdHealthMap[gvk].MarkWorkersStopped()
		// k.crdHealthMap[gvk].workerStates.Range(func(key, value interface{}) bool {
		// 	k.crdHealthMap[gvk].workerStates.Store(key, crdhealth.WorkerStateStopped)
		// 	return true
		// })

		dc.deactivated[gvk] = true
		logger.Info().Str("gvk", gvk).Msg("workers drained cleanly")
	case <-time.After(drainTimeout):
		logger.Warn().Str("gvk", gvk).Msg("drain timeout exceeded")
	}
}

// crdExists checks if a CRD is present in the cluster by querying the API server.
//
// Return values:
//   - (true,  nil) — CRD exists
//   - (false, nil) — CRD definitively not registered ("not installed")
//   - (false, err) — transient failure (network blip, timeout, dial error);
//     callers must skip all health-state changes and retry next tick
func (dc *DependencyCoordinator) crdExists(gvk *schema.GroupVersionKind) (bool, error) {
	_, err := kube.CheckCRDExists(dc.kube.RestConfig(), gvk)
	if err == nil {
		return true, nil
	}
	// WaitForCRD converts meta.IsNoMatchError into "not installed" — that is the
	// only definitive "CRD absent" signal. Everything else (dial, timeout, TLS, …)
	// is a transient error; don't degrade CRD health based on it.
	if strings.Contains(err.Error(), "not installed") {
		return false, nil
	}
	return false, err
}

// collectCustomChildGVKs scans all registered CRDs for custom resource entries declared
// in onCreate.custom / onReconcile.custom and returns a de-duped map of GVK string → GVK.
func collectCustomChildGVKs(catalog *ResourceCatalog) map[string]schema.GroupVersionKind {
	result := make(map[string]schema.GroupVersionKind)
	for _, entry := range catalog.Entries() {
		box := entry.CRD.OperatorBox
		var srcs []types.CustomResourceTemplateSource
		if box.EffectiveOnCreate() != nil {
			srcs = append(srcs, box.EffectiveOnCreate().CustomResource...)
		}
		if box.EffectiveOnReconcile() != nil {
			srcs = append(srcs, box.EffectiveOnReconcile().CustomResource...)
		}
		for i := range srcs {
			gvk, err := srcs[i].BuildGVK()
			if err != nil {
				continue
			}
			result[gvk.String()] = gvk
		}
	}
	return result
}
