// Package app assembles the Inrun gateway. Everywhere it serves the Gateway
// API and runs schema migrations; inside a pod it also sets up TLS, the
// webhook server with its /blueprint routes, and the housekeeper that keeps
// the gateway's infrastructure in sync. New wires everything and starts
// nothing; the caller starts the process. The gateway runs no reconcilers,
// informers or leader election: it is stateless and runs as several replicas.
// The replicas take a Lease only to run the startup migration one at a time.
package app
