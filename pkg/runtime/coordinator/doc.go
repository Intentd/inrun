// Package coordinator decides when each CRD's workers start, in dependency
// order, and runs them: each worker takes keys off the CRD's queue and hands
// them to reconcile.Run for one pass. It also activates CRDs that appear after
// startup and restarts those that are deleted and recreated.
package coordinator
