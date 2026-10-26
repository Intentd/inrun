// Package queue buffers informer events for the reconciler. Each CRD gets a
// Workqueue that holds keys, applies the declared queue behaviour and knows
// nothing about what is reconciled.
package queue
