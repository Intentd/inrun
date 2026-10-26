// Package metrics registers the Prometheus metrics served at /metrics.
// Importing the package registers them; callers record values only through
// its Record*, Observe* and Set* functions.
package metrics
