package config

import "time"

// Port returns the configured port for the health server.
func (h *healthServer) Port() string {
	return h.port
}

// SetPort sets the configured port for the health server.
func (h *healthServer) SetPort(v string) {
	h.port = v
}

// ReadTimeout returns the read timeout for the health server.
func (h *healthServer) ReadTimeout() time.Duration {
	return h.readTimeout
}

// SetReadTimeout sets the read timeout for the health server.
func (h *healthServer) SetReadTimeout(v time.Duration) {
	h.readTimeout = v
}

// WriteTimeout returns the write timeout for the health server.
func (h *healthServer) WriteTimeout() time.Duration {
	return h.writeTimeout
}

// SetWriteTimeout sets the write timeout for the health server.
func (h *healthServer) SetWriteTimeout(v time.Duration) {
	h.writeTimeout = v
}

// KubeconfigPath returns the path to the kubeconfig file for the cluster.
func (c *clusterConfig) KubeconfigPath() string {
	return c.kubeconfigPath
}

// SetKubeconfigPath sets the path to the kubeconfig file for the cluster.
func (c *clusterConfig) SetKubeconfigPath(v string) {
	c.kubeconfigPath = v
}

// MasterURL returns the API server master URL for the cluster.
func (c *clusterConfig) MasterURL() string {
	return c.masterURL
}

// SetMasterURL sets the API server master URL for the cluster.
func (c *clusterConfig) SetMasterURL(v string) {
	c.masterURL = v
}

// Name returns the logical name of the cluster.
func (c *clusterConfig) Name() string {
	return c.name
}

// SetName sets the logical name of the cluster.
func (c *clusterConfig) SetName(v string) {
	c.name = v
}

// Namespace returns the namespace used by the cluster components.
func (c *clusterConfig) Namespace() string {
	return c.namespace
}

// SetNamespace sets the namespace used by the cluster components.
func (c *clusterConfig) SetNamespace(v string) {
	c.namespace = v
}
