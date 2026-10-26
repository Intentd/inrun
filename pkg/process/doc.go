// Package process starts and stops the components that run inside one
// Inrun process (HTTP servers, webhook servers, clients, the reconcile
// loop). New creates a Manager with its components; Start runs them
// and blocks until the context is cancelled or SIGINT or SIGTERM arrives. On a
// signal it stops them in reverse start order within the configured timeout.
package process
