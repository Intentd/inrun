// Package registry pushes and pulls operator patterns and modules as OCI
// artifacts. It resolves references, detects the artifact kind from its
// primary YAML file, caches pulls and lists what a registry holds. Use
// NewClient to talk to a registry.
package registry
