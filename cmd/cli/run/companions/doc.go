//go:build !runtime && !gateway

// Package companions starts the processes that run next to a local inrun:
// the console always, and the gateway when the Blueprint serves. They run as
// child processes and stop with inrun.
package companions
