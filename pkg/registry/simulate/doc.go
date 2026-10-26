// Package simulate runs an operator's reconcile loop for a Blueprint and a
// CR, in memory or on envtest and records what it writes.
// Evaluate checks the result against simulate.yaml;
// Compare checks an in-memory run against an envtest run.
package simulate
