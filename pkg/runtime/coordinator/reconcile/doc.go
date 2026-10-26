// Package reconcile runs one reconcile pass for one key: the pre-reconcile
// gate, prepare, cleanup and maintain, the reconciler itself, finalizing, apply
// and prune, the surface sweep, post, and the next requeue. The coordinator owns
// the workers and queues; it calls Run for each key it dequeues and acts on the
// Outcome.
package reconcile
