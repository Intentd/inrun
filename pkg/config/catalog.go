package config

import "time"

// catalog.go provides accessor methods for the unexported catalogConfig
// fields. Callers should use these methods to read and mutate catalog
// configuration without exposing the underlying struct fields.
//
// Example usage:
//   cfg.Catalog().DefaultWorkers()
//   cfg.Catalog().ShutdownTimeout()
//
// The underlying catalogConfig struct is expected to use unexported field
// names (paths, defaultQueueDepth, defaultFailureThreshold, defaultResync,
// defaultWorkers, shutdownTimeout, shutdownGracePeriod).
//
// This file intentionally contains only simple, side-effect-free accessors
// and a small set of helpers for common operations.

// Paths returns the configured catalog file paths.
func (c *catalogConfig) Paths() []string {
	return c.paths
}

// HasPaths reports whether any catalog paths have been configured.
func (c *catalogConfig) HasPaths() bool {
	return len(c.paths) > 0
}

// AddPath appends a catalog path to the configuration.
func (c *catalogConfig) AddPath(path string) {
	if path == "" {
		return
	}
	c.paths = append(c.paths, path)
}

// DefaultQueueDepth returns the defaultmaximum  queue depth for CRD workers.
func (c *catalogConfig) DefaultQueueDepth() int {
	return c.defaultQueueDepth
}

// DefaultFailureThreshold returns the default degrade threshold.
func (c *catalogConfig) DefaultFailureThreshold() int {
	return c.defaultFailureThreshold
}

// DefaultResync returns the default informer resync period.
func (c *catalogConfig) DefaultResync() time.Duration {
	return c.defaultResync
}

// DefaultWorkers returns the default number of reconcile workers per CRD.
func (c *catalogConfig) DefaultWorkers() int {
	return c.defaultWorkers
}

// ShutdownTimeout returns the hard timeout used during shutdown.
func (c *catalogConfig) ShutdownTimeout() time.Duration {
	return c.shutdownTimeout
}

// ShutdownGracePeriod returns the grace period before forced shutdown.
func (c *catalogConfig) ShutdownGracePeriod() time.Duration {
	return c.shutdownGracePeriod
}

// GatewayEndpoint returns the advertised gateway endpoint; it may be empty
// when no gateway is configured.
func (c *catalogConfig) GatewayEndpoint() string {
	return c.gatewayEndpoint
}
