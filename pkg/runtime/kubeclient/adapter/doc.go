// Package adapter is the boundary between controller-runtime and the
// Inrun runtime. ToClient exposes a kubeclient as a controller-runtime
// client.Client, so reconcilers written for controller-runtime run unchanged
// while Inrun owns informers, queues, workers and scheduling.
package adapter
