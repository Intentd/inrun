// Package app assembles the Inrun runtime: the Blueprint, Kubernetes clients and
// informers, every CRD's queue, informer and reconciler, the HTTP routes, the
// coordinator, and the process that starts and stops them. New wires all of it
// and starts nothing; the caller starts the process. Being importable, it lets
// tests and tools run a real runtime in-process.
//
// Only one runtime pod reconciles at a time: NewLeaderElection holds a Lease,
// and the pod that wins it runs the workers while one that loses it stops them.
package app
