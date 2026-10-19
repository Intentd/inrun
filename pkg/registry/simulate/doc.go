// Package simulate runs an operator's reconcile loop against an in-memory
// fake cluster and reports the resources it creates, updates or deletes.
// Run takes a Blueprint and a CR; Assert checks the result against the
// expectations in simulate.yaml.
package simulate
