// Package post runs after the reconciler returns: it writes the Ready
// condition and declared status fields and emits declared events. Every step
// is best effort and never requeues.
package post
