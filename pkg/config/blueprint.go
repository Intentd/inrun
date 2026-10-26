package config

import "time"

// blueprint.go provides accessor methods for the unexported blueprintConfig
// fields. Callers should use these methods to read and mutate blueprint
// configuration without exposing the underlying struct fields.

// Paths returns the configured blueprint file paths.
func (c *blueprintConfig) Paths() []string {
	return c.paths
}

// HasPaths reports whether any blueprint paths have been configured.
func (c *blueprintConfig) HasPaths() bool {
	return len(c.paths) > 0
}

// AddPath appends a blueprint path to the configuration.
func (c *blueprintConfig) AddPath(path string) {
	if path == "" {
		return
	}
	c.paths = append(c.paths, path)
}

// DefaultQueueDepth returns the defaultmaximum  queue depth for CRD workers.
func (c *blueprintConfig) DefaultQueueDepth() int {
	return c.defaultQueueDepth
}

// DefaultFailureThreshold returns the default degrade threshold.
func (c *blueprintConfig) DefaultFailureThreshold() int {
	return c.defaultFailureThreshold
}

// DefaultResync returns the default informer resync period.
func (c *blueprintConfig) DefaultResync() time.Duration {
	return c.defaultResync
}

// DefaultWorkers returns the default number of reconcile workers per CRD.
func (c *blueprintConfig) DefaultWorkers() int {
	return c.defaultWorkers
}

// ShutdownTimeout returns the hard timeout used during shutdown.
func (c *blueprintConfig) ShutdownTimeout() time.Duration {
	return c.shutdownTimeout
}

// ShutdownGracePeriod returns the grace period before forced shutdown.
func (c *blueprintConfig) ShutdownGracePeriod() time.Duration {
	return c.shutdownGracePeriod
}

// GatewayEndpoint returns the advertised gateway endpoint; it may be empty
// when no gateway is configured.
func (c *blueprintConfig) GatewayEndpoint() string {
	return c.gatewayEndpoint
}
